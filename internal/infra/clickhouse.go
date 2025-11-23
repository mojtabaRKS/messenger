package infra

import (
	"arvan/message-gateway/internal/config"
	"fmt"
	"github.com/golang-migrate/migrate/v4"
	migrateCk "github.com/golang-migrate/migrate/v4/database/clickhouse"
	"github.com/pkg/errors"
	gormLogger "gorm.io/gorm/logger"
	"log"
	"os"
	"time"

	stdCk "github.com/ClickHouse/clickhouse-go/v2"
	"gorm.io/driver/clickhouse"
	"gorm.io/gorm"
)

type ClickHouseClient struct {
	db *gorm.DB
}

func NewClickHouseClient(cfg config.ClickHouse) (*ClickHouseClient, error) {
	newLogger := gormLogger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags), // io writer
		gormLogger.Config{
			SlowThreshold:             2 * time.Second,
			LogLevel:                  gormLogger.Info,
			IgnoreRecordNotFoundError: true, // Ignore ErrRecordNotFound error for logger
			Colorful:                  true,
		},
	)

	db, err := gorm.Open(clickhouse.New(clickhouse.Config{
		Conn: stdCk.OpenDB(&stdCk.Options{
			Addr: []string{fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)},
			Auth: stdCk.Auth{
				Database: cfg.Database,
				Username: cfg.Username,
				Password: cfg.Password,
			},
			Settings: stdCk.Settings{
				"max_execution_time": 60,
			},
			DialTimeout:      time.Duration(10) * time.Second,
			MaxOpenConns:     5,
			MaxIdleConns:     5,
			ConnMaxLifetime:  time.Duration(10) * time.Minute,
			ConnOpenStrategy: stdCk.ConnOpenInOrder,
		}),
	}), &gorm.Config{Logger: newLogger})
	if err != nil {
		return nil, err
	}

	return &ClickHouseClient{db: db}, nil
}

func (c *ClickHouseClient) GetDb() *gorm.DB {
	return c.db
}

func (c *ClickHouseClient) MigrateUp(dbName string) error {
	m, err := c.prepareConnection(dbName)
	if err != nil {
		return err
	}

	if err := m.Up(); err != nil {
		return err
	}

	return nil
}

func (c *ClickHouseClient) MigrateDown(dbName string) error {
	m, err := c.prepareConnection(dbName)
	if err != nil {
		return err
	}

	if err := m.Down(); err != nil {
		return err
	}

	return nil
}

func (c *ClickHouseClient) prepareConnection(dbName string) (*migrate.Migrate, error) {
	conn, err := c.db.DB()
	if err != nil {
		return nil, err
	}

	driver, err := migrateCk.WithInstance(conn, &migrateCk.Config{})
	if err != nil {
		return nil, err
	}

	m, err := migrate.NewWithDatabaseInstance("file://migrations/clickhouse", dbName, driver)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create migrations instance")
	}
	return m, nil

}
