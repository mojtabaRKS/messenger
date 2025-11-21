package balance

import (
	"arvan/message-gateway/internal/constant"
	"arvan/message-gateway/internal/repository/entity"
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"gorm.io/gorm"
)

func (bs *BalanceService) DeductBalanceAndQueueSms(
	ctx context.Context,
	customerId int,
	message, receiver string,
) (uuid.UUID, error) {
	msgId := uuid.New()

	balanceKey := fmt.Sprintf("%s%d", constant.BalanceKeyPrefix, customerId)

	result, err := bs.deductScript.Run(ctx, bs.redisClient, []string{balanceKey}, 10).Result()
	if err != nil {
		bs.logger.Errorf("redis balance deduction failed for customer %d: %v", customerId, err)
		return uuid.Nil, errors.Wrap(err, "failed to deduct balance from redis")
	}

	newBalance, ok := result.(int64)
	if !ok {
		bs.logger.Errorf("unexpected redis result type for customer %d: %T", customerId, result)
		return uuid.Nil, errors.New("unexpected redis result type")
	}

	if newBalance < 0 {
		return uuid.Nil, constant.InsufficientBalanceErr
	}

	update := &BalanceUpdate{
		MsgID:      msgId,
		CustomerID: customerId,
		ToNumber:   receiver,
		Body:       message,
		Timestamp:  time.Now().UTC(),
	}

	select {
	case bs.pendingWrites <- update:
	default:
		bs.logger.Warnf("batch write queue full, may have delayed persistence for customer %d", customerId)
		go bs.writeSingleUpdate(update)
	}

	return msgId, nil
}

// InitializeBalanceCache loads all customer balances from DB into Redis on startup
func (bs *BalanceService) InitializeBalanceCache(ctx context.Context) error {
	bs.logger.Info("initializing balance cache from database...")

	var balances []entity.Balance
	if err := bs.db.WithContext(ctx).Find(&balances).Error; err != nil {
		return errors.Wrap(err, "failed to load balances from database")
	}

	// Load into Redis with pipeline for efficiency
	pipe := bs.redisClient.Pipeline()
	count := 0

	for _, bal := range balances {
		balanceKey := fmt.Sprintf("%s%d", constant.BalanceKeyPrefix, bal.CustomerId)
		pipe.Set(ctx, balanceKey, bal.BalanceBigint, 0) // 0 = no expiration
		count++

		// Execute pipeline in batches of 1000
		if count%1000 == 0 {
			if _, err := pipe.Exec(ctx); err != nil {
				bs.logger.Errorf("failed to execute redis pipeline: %v", err)
			}
			pipe = bs.redisClient.Pipeline()
		}
	}

	// Execute remaining
	if count%1000 != 0 {
		if _, err := pipe.Exec(ctx); err != nil {
			return errors.Wrap(err, "failed to execute final redis pipeline")
		}
	}

	bs.logger.Infof("initialized %d customer balances in Redis cache", len(balances))
	return nil
}

// batchWriter runs in background and periodically flushes pending writes to database
func (bs *BalanceService) batchWriter(workerID int) {
	defer bs.wg.Done()

	ticker := time.NewTicker(constant.BalanceSyncInterval)
	defer ticker.Stop()

	batch := make([]*BalanceUpdate, 0, constant.BalanceSyncBatchSize)

	bs.logger.Infof("balance batch writer %d started", workerID)

	for {
		select {
		case <-bs.stopCh:
			// Flush remaining on shutdown
			bs.logger.Infof("balance batch writer %d: flushing remaining writes on shutdown...", workerID)
			bs.flushBatch(batch, workerID)
			// Drain channel
			for len(bs.pendingWrites) > 0 {
				update := <-bs.pendingWrites
				batch = append(batch, update)
				if len(batch) >= constant.BalanceSyncBatchSize {
					bs.flushBatch(batch, workerID)
					batch = batch[:0]
				}
			}
			bs.flushBatch(batch, workerID)
			bs.logger.Infof("balance batch writer %d: stopped", workerID)
			return

		case update := <-bs.pendingWrites:
			batch = append(batch, update)

			// Flush if batch is full
			if len(batch) >= constant.BalanceSyncBatchSize {
				bs.flushBatch(batch, workerID)
				batch = batch[:0]
			}

		case <-ticker.C:
			// Periodic flush
			if len(batch) > 0 {
				bs.flushBatch(batch, workerID)
				batch = batch[:0]
			}
		}
	}
}

// flushBatch writes a batch of updates to database in a single transaction
func (bs *BalanceService) flushBatch(batch []*BalanceUpdate, workerID int) {
	if len(batch) == 0 {
		return
	}

	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := bs.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Batch insert SMS logs
		smsLogs := make([]entity.SmsLog, len(batch))
		for i, update := range batch {
			smsLogs[i] = entity.SmsLog{
				MessageId:  update.MsgID,
				CustomerId: update.CustomerID,
				ToNumber:   update.ToNumber,
				Body:       update.Body,
				CreatedAt:  update.Timestamp,
			}
		}

		// Batch insert
		if err := tx.CreateInBatches(smsLogs, 500).Error; err != nil {
			return errors.Wrap(err, "failed to batch insert sms logs")
		}

		// Update balances: sync Redis state to DB
		// Group by customer ID for efficient updates
		customerIDs := make(map[int]bool) // customerID set
		for _, update := range batch {
			customerIDs[update.CustomerID] = true
		}

		// Update each customer's balance
		for customerID := range customerIDs {
			// Get current balance from Redis as source of truth
			balanceKey := fmt.Sprintf("%s%d", constant.BalanceKeyPrefix, customerID)
			redisBalance, err := bs.redisClient.Get(context.Background(), balanceKey).Int64()
			if err != nil {
				bs.logger.Warnf("failed to get redis balance for customer %d: %v", customerID, err)
				continue
			}

			// Update DB to match Redis
			if err := tx.Model(&entity.Balance{}).
				Where("customer_id = ?", customerID).
				Update("balance_bigint", redisBalance).Error; err != nil {
				return errors.Wrapf(err, "failed to update balance for customer %d", customerID)
			}
		}

		return nil
	})

	elapsed := time.Since(start)

	if err != nil {
		bs.logger.Errorf("batch writer %d: write failed (%d records, %v elapsed): %v", workerID, len(batch), elapsed, err)
		// TODO: retry logic or DLQ for failed batches
	} else {
		bs.logger.Infof("batch writer %d: successful (%d records synced to DB in %v)", workerID, len(batch), elapsed)
	}
}

// writeSingleUpdate writes a single update synchronously (fallback when queue is full)
func (bs *BalanceService) writeSingleUpdate(update *BalanceUpdate) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := bs.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Insert SMS log
		smsLog := entity.SmsLog{
			MessageId:  update.MsgID,
			CustomerId: update.CustomerID,
			ToNumber:   update.ToNumber,
			Body:       update.Body,
			CreatedAt:  update.Timestamp,
		}

		if err := tx.Create(&smsLog).Error; err != nil {
			return errors.Wrap(err, "failed to insert sms log")
		}

		// Sync balance from Redis
		balanceKey := fmt.Sprintf("%s%d", constant.BalanceKeyPrefix, update.CustomerID)
		redisBalance, err := bs.redisClient.Get(context.Background(), balanceKey).Int64()
		if err != nil {
			return errors.Wrap(err, "failed to get redis balance")
		}

		if err := tx.Model(&entity.Balance{}).
			Where("customer_id = ?", update.CustomerID).
			Update("balance_bigint", redisBalance).Error; err != nil {
			return errors.Wrap(err, "failed to update balance")
		}

		return nil
	})

	if err != nil {
		bs.logger.Errorf("single write failed for customer %d: %v", update.CustomerID, err)
	}
}
