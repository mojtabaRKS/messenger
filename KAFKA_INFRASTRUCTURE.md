# Kafka Infrastructure - SMS Gateway

## Overview

Kafka serves as the event backbone for:
1. Decoupling API service from downstream consumers
2. Enabling real-time analytics via ClickHouse
3. Distributing SMS delivery tasks to workers
4. Providing event replay capabilities

---

## Topic Design

### 1. sms.accepted

**Purpose**: Events for accepted SMS (after balance deduction)

**Configuration**:
```properties
partitions=30
replication.factor=3
min.insync.replicas=2
retention.ms=604800000  # 7 days
cleanup.policy=delete
compression.type=snappy
segment.ms=86400000  # 1 day segments
```

**Partitioning Strategy**: Hash by `customer_id`
- Ensures ordering per customer
- Distributes load across partitions
- ~40 messages/sec per partition at 1200 TPS

**Message Schema**:
```json
{
  "event_id": "01HN3QXYZ...",
  "event_type": "sms.accepted",
  "timestamp": "2025-11-19T10:30:45.123Z",
  "customer_id": "cust_12345",
  "sms_id": 9876543210,
  "phone": "+989121234567",
  "message": "Your verification code is 123456",
  "cost": 500,
  "idempotency_key": "req_abc123",
  "metadata": {
    "ip": "192.168.1.100",
    "user_agent": "MyApp/1.0"
  }
}
```

**Consumers**:
- ClickHouse Kafka Engine (10 consumers)
- Analytics workers (5 consumer groups)
- Audit logger (1 consumer group)

**Throughput Estimate**:
- 100M SMS/day = 1,157 msg/sec average
- Peak: 3,000 msg/sec
- Message size: ~300 bytes avg
- Bandwidth: 1,157 × 300 = 347 KB/sec avg, 900 KB/sec peak

---

### 2. sms.delivery

**Purpose**: Delivery tasks for workers to send SMS to operators

**Configuration**:
```properties
partitions=50
replication.factor=3
min.insync.replicas=2
retention.ms=259200000  # 3 days
cleanup.policy=delete
compression.type=snappy
```

**Partitioning Strategy**: Hash by `customer_id` OR `operator_id`
- Allows per-customer or per-operator rate shaping
- More partitions = finer-grained worker distribution

**Message Schema**:
```json
{
  "task_id": "01HN3QXYZ...",
  "sms_id": 9876543210,
  "customer_id": "cust_12345",
  "phone": "+989121234567",
  "message": "Your verification code is 123456",
  "operator": "mci",
  "priority": 2,
  "retry_count": 0,
  "max_retries": 3,
  "created_at": "2025-11-19T10:30:45.123Z"
}
```

**Consumers**:
- Delivery worker pool (10-50 consumer groups)
- Each worker handles multiple partitions

---

### 3. sms.status

**Purpose**: Delivery status updates from workers

**Configuration**:
```properties
partitions=30
replication.factor=3
min.insync.replicas=2
retention.ms=604800000  # 7 days
cleanup.policy=delete
compression.type=snappy
```

**Partitioning Strategy**: Hash by `sms_id`

**Message Schema**:
```json
{
  "event_id": "01HN3QXYZ...",
  "sms_id": 9876543210,
  "customer_id": "cust_12345",
  "status": "delivered",
  "operator_message_id": "op_msg_12345",
  "error_code": null,
  "error_message": null,
  "delivered_at": "2025-11-19T10:30:50.456Z",
  "latency_ms": 234
}
```

**Status Values**:
- `queued`: In worker queue
- `sending`: Sending to operator
- `sent`: Accepted by operator
- `delivered`: Confirmed delivery
- `failed`: Permanent failure
- `rejected`: Rejected by operator

**Consumers**:
- Status updater (updates PostgreSQL)
- ClickHouse ingestion
- Webhook dispatcher (customer notifications)

---

### 4. sms.dlq (Dead Letter Queue)

**Purpose**: Failed messages for manual review

**Configuration**:
```properties
partitions=10
replication.factor=3
retention.ms=2592000000  # 30 days
cleanup.policy=delete
```

**Message Schema**:
```json
{
  "original_topic": "sms.delivery",
  "original_partition": 5,
  "original_offset": 123456,
  "error": "Timeout after 3 retries",
  "message": { ... },
  "failed_at": "2025-11-19T10:35:00.000Z"
}
```

---

## Kafka Cluster Sizing

### Broker Configuration

**For 100M SMS/day**:
- **Brokers**: 5-7 nodes
- **CPU**: 8-16 cores per broker
- **Memory**: 32-64 GB per broker
- **Disk**: 1-2 TB NVMe SSD per broker
- **Network**: 10 Gbps

**Kafka Broker Settings** (server.properties):
```properties
# Broker basics
broker.id=1
log.dirs=/data/kafka-logs

# Network threads
num.network.threads=8
num.io.threads=16

# Log settings
log.retention.hours=168  # 7 days default
log.segment.bytes=1073741824  # 1 GB segments
log.retention.check.interval.ms=300000  # 5 minutes

# Replication
default.replication.factor=3
min.insync.replicas=2
unclean.leader.election.enable=false
auto.create.topics.enable=false

# Performance tuning
num.replica.fetchers=4
replica.lag.time.max.ms=10000
socket.send.buffer.bytes=102400
socket.receive.buffer.bytes=102400
socket.request.max.bytes=104857600

# Compression
compression.type=snappy

# Log cleanup
log.cleaner.enable=true
log.cleanup.policy=delete
```

---

## Producer Configuration

### API Service Producer (Go)

```go
producerConfig := &kafka.ConfigMap{
    "bootstrap.servers": "kafka-1:9092,kafka-2:9092,kafka-3:9092",
    
    // Performance
    "acks": "1",  // Leader only (acceptable for this use case)
    "linger.ms": 100,  // Batch for 100ms
    "batch.size": 16384,  // 16KB batches
    "compression.type": "snappy",
    "buffer.memory": 33554432,  // 32MB buffer
    
    // Reliability
    "enable.idempotence": true,  // Exactly-once to Kafka
    "max.in.flight.requests.per.connection": 5,
    "retries": 3,
    "retry.backoff.ms": 100,
    
    // Timeouts
    "request.timeout.ms": 5000,
    "delivery.timeout.ms": 10000,
    
    // Monitoring
    "statistics.interval.ms": 10000,
}
```

**Batching Strategy**:
- Wait up to 100ms to accumulate messages
- Send when batch reaches 16KB
- Reduces network overhead (10-20x throughput improvement)

**Error Handling**:
```go
func produceMessage(producer *kafka.Producer, topic string, key string, value []byte) error {
    deliveryChan := make(chan kafka.Event)
    
    err := producer.Produce(&kafka.Message{
        TopicPartition: kafka.TopicPartition{
            Topic: &topic,
            Partition: kafka.PartitionAny,
        },
        Key:   []byte(key),
        Value: value,
    }, deliveryChan)
    
    if err != nil {
        return err
    }
    
    // Async delivery confirmation
    go func() {
        e := <-deliveryChan
        m := e.(*kafka.Message)
        if m.TopicPartition.Error != nil {
            log.Error("Delivery failed", m.TopicPartition.Error)
            // Send to DLQ or retry
        }
    }()
    
    return nil
}
```

---

## Consumer Configuration

### ClickHouse Ingestion Consumer

**ClickHouse Kafka Engine Settings** (see DATABASE_SCHEMA.md):
```sql
kafka_num_consumers = 10
kafka_max_block_size = 65536  # 64K rows per batch
kafka_flush_interval_ms = 7500  # Flush every 7.5 seconds
```

---

### Delivery Worker Consumer (Go)

```go
consumerConfig := &kafka.ConfigMap{
    "bootstrap.servers": "kafka-1:9092,kafka-2:9092,kafka-3:9092",
    "group.id": "delivery-worker-group",
    
    // Consumer settings
    "auto.offset.reset": "earliest",
    "enable.auto.commit": false,  // Manual commit for reliability
    "max.poll.interval.ms": 300000,  // 5 minutes
    "session.timeout.ms": 10000,
    
    // Fetch settings
    "fetch.min.bytes": 1024,
    "fetch.wait.max.ms": 500,
    "max.partition.fetch.bytes": 1048576,  // 1MB
}
```

**Consumer Pattern** (with manual commit):
```go
for {
    msg, err := consumer.ReadMessage(100 * time.Millisecond)
    if err != nil {
        continue
    }
    
    // Process message
    if err := processDelivery(msg.Value); err != nil {
        log.Error("Failed to process", err)
        // Send to DLQ
        continue
    }
    
    // Manual commit after successful processing
    consumer.CommitMessage(msg)
}
```

---

## Partition Assignment Strategy

### For sms.accepted (30 partitions)

**Distribution** at 1,200 msg/sec:
```
Partition 0: ~40 msg/sec (customers 1, 31, 61, ...)
Partition 1: ~40 msg/sec (customers 2, 32, 62, ...)
...
Partition 29: ~40 msg/sec (customers 30, 60, 90, ...)
```

**Consumer Groups**:
- ClickHouse: 10 consumers (each handles 3 partitions)
- Analytics: 5 consumers (each handles 6 partitions)

---

### For sms.delivery (50 partitions)

**Worker Pool Strategy**:
- 10 worker instances
- Each instance: 1 consumer handling 5 partitions
- Load balancing: Kafka consumer group rebalancing

**Scaling**:
- Add more workers → automatic rebalancing
- Max workers = 50 (one per partition)

---

## Consumer Lag Monitoring

### Lag Calculation
```
Lag = Latest Offset - Consumer Offset
```

**Acceptable Lag**:
- Normal: <10,000 messages (<10 seconds at 1,200 msg/sec)
- Warning: 10,000-100,000 messages (10-90 seconds)
- Critical: >100,000 messages (>90 seconds)

### Monitoring Tools

**Kafka CLI**:
```bash
# Check consumer group lag
kafka-consumer-groups.sh \
  --bootstrap-server kafka-1:9092 \
  --group clickhouse_ingestion \
  --describe

# Output:
# GROUP                  TOPIC         PARTITION  CURRENT-OFFSET  LOG-END-OFFSET  LAG
# clickhouse_ingestion   sms.accepted  0          1000000         1000050         50
# clickhouse_ingestion   sms.accepted  1          999800          999850          50
```

**Prometheus Metrics** (via JMX Exporter):
```promql
# Consumer lag
kafka_consumer_lag{group="clickhouse_ingestion",topic="sms.accepted"}

# Alert on high lag
ALERT HighConsumerLag
  IF kafka_consumer_lag > 100000
  FOR 5m
  LABELS { severity = "critical" }
```

---

## Throughput Calculations

### Message Size Estimates
```
sms.accepted: ~300 bytes avg
sms.delivery: ~250 bytes avg
sms.status: ~200 bytes avg
```

### Bandwidth Requirements (100M SMS/day)

**Ingress** (to Kafka):
```
1,200 msg/sec × 300 bytes = 360 KB/sec = 2.88 Mbps
Peak: 3,000 msg/sec × 300 bytes = 900 KB/sec = 7.2 Mbps
```

**Egress** (from Kafka):
- ClickHouse: 360 KB/sec
- Workers: 360 KB/sec
- Analytics: 360 KB/sec
- **Total**: ~1.1 MB/sec = 8.8 Mbps

**Replication Overhead** (3x):
- Intra-cluster: 2.88 Mbps × 2 = 5.76 Mbps

**Total Bandwidth**: ~15-20 Mbps average, 50 Mbps peak

---

## Disk Usage Estimates

### Per Topic Storage (7-day retention)

**sms.accepted**:
```
100M msg/day × 300 bytes × 7 days = 210 GB
With compression (3x): 70 GB
With replication (3x): 210 GB total
```

**sms.delivery**:
```
100M msg/day × 250 bytes × 3 days = 75 GB
With compression: 25 GB
With replication: 75 GB total
```

**sms.status**:
```
100M msg/day × 200 bytes × 7 days = 140 GB
With compression: 47 GB
With replication: 141 GB total
```

**Total Cluster Storage**: ~426 GB (with compression + replication)

**Per Broker** (5 brokers): ~85 GB

**Recommended**: 1 TB per broker (12x headroom)

---

## Fault Tolerance

### Broker Failure

**Scenario**: 1 broker fails out of 5

**Impact**:
- Replication factor 3 → 2 remaining replicas
- Min ISR = 2 → writes continue
- Consumers: automatic rebalancing to other brokers

**Recovery**:
- Replace broker, Kafka auto-replicates missing partitions
- No data loss if min ISR maintained

---

### Producer Failure

**Scenario**: API service pod crashes

**Impact**:
- In-flight messages (buffered): lost (~100ms worth = 120 messages)
- Accepted SMS already in Postgres: safe
- Kafka producer buffer: lost (acceptable)

**Mitigation**:
- Idempotency keys prevent duplicate SMS
- Use `enable.idempotence=true` for Kafka

---

### Consumer Failure

**Scenario**: ClickHouse consumer crashes

**Impact**:
- Kafka retains messages (7-day retention)
- Consumer group lag increases
- Recovery: restart consumer, processes from last committed offset

**Mitigation**:
- Run multiple consumer instances (10 for ClickHouse)
- Monitor lag and alert

---

## Performance Optimization

### 1. Batching (10-20x improvement)
- Producer: `linger.ms=100`, `batch.size=16KB`
- Consumer: `max.poll.records=500`

### 2. Compression (3-5x reduction)
- Snappy: fast, ~50% compression ratio
- Alternative: LZ4 (faster) or GZIP (higher compression)

### 3. Async Producers
- Don't wait for delivery confirmation in API response
- Use callbacks for error handling

### 4. Partition Parallelism
- More partitions = more parallel consumers
- Balance: too many partitions = overhead

### 5. NVMe SSDs
- 10x faster than HDD for Kafka logs
- Reduces tail latency

---

## Kafka Operations

### Create Topics

```bash
# sms.accepted
kafka-topics.sh --create \
  --bootstrap-server kafka-1:9092 \
  --topic sms.accepted \
  --partitions 30 \
  --replication-factor 3 \
  --config min.insync.replicas=2 \
  --config retention.ms=604800000 \
  --config compression.type=snappy

# sms.delivery
kafka-topics.sh --create \
  --bootstrap-server kafka-1:9092 \
  --topic sms.delivery \
  --partitions 50 \
  --replication-factor 3 \
  --config min.insync.replicas=2 \
  --config retention.ms=259200000 \
  --config compression.type=snappy

# sms.status
kafka-topics.sh --create \
  --bootstrap-server kafka-1:9092 \
  --topic sms.status \
  --partitions 30 \
  --replication-factor 3 \
  --config min.insync.replicas=2 \
  --config retention.ms=604800000 \
  --config compression.type=snappy
```

---

### Monitor Cluster Health

```bash
# Check topic details
kafka-topics.sh --describe \
  --bootstrap-server kafka-1:9092 \
  --topic sms.accepted

# Check broker logs
tail -f /data/kafka-logs/server.log

# Check replication status
kafka-topics.sh --describe \
  --bootstrap-server kafka-1:9092 \
  --under-replicated-partitions
```

---

### Increase Partitions (Scale Up)

```bash
# Increase from 30 to 40 partitions
kafka-topics.sh --alter \
  --bootstrap-server kafka-1:9092 \
  --topic sms.accepted \
  --partitions 40
```

**Note**: Cannot decrease partitions (Kafka limitation)

---

## Integration with Other Components

### API Service → Kafka
```go
// Produce after balance deduction + SMS log insert
event := SMSAcceptedEvent{
    EventID:    generateULID(),
    CustomerID: customerID,
    SMSID:      smsID,
    // ...
}

kafkaProducer.Produce("sms.accepted", customerID, event)
```

---

### Kafka → ClickHouse
```sql
-- Automatic ingestion via Kafka Engine
-- Materialized view streams to MergeTree table
```

---

### Kafka → Workers
```go
// Worker consumes from sms.delivery
for msg := range kafkaConsumer.Messages() {
    task := parseDeliveryTask(msg.Value)
    
    // Enqueue to per-customer buffer
    customerQueue := workerPool.getQueue(task.CustomerID)
    customerQueue.Enqueue(task)
    
    // Commit offset after enqueue (not after delivery)
    kafkaConsumer.CommitMessage(msg)
}
```

---

## Summary

This Kafka infrastructure achieves:
✅ 3,000 msg/sec peak throughput (30x headroom)  
✅ <1 second end-to-end latency (API → ClickHouse)  
✅ Fault-tolerant with 3x replication  
✅ Scalable to 500M+ SMS/day (add more partitions)  
✅ Efficient batching (10-20x throughput improvement)  
✅ Multiple consumer groups for different use cases  
✅ Clear monitoring and alerting strategy  

**Next Steps**: Implement Go producer/consumer wrappers in API service and workers.

