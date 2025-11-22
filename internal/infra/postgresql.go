package infra

import (
	"arvan/message-gateway/internal/config"
	"context"
	"fmt"

	"github.com/pkg/errors"

	"github.com/golang-migrate/migrate/v4"
	migratePsql "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type PostgresClient struct {
	db *gorm.DB
}

func NewPostgresClient(ctx context.Context, cfg config.Postgres) (*PostgresClient, error) {
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%d sslmode=disable",
		cfg.Host,
		cfg.Username,
		cfg.Password,
		cfg.Database,
		cfg.Port,
	)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	return &PostgresClient{db: db.WithContext(ctx)}, nil
}

func (p *PostgresClient) GetDb() *gorm.DB {
	return p.db
}

func (p *PostgresClient) MigrateUp(dbName string) error {
	m, err := p.prepareConnection(dbName)
	if err != nil {
		return err
	}

	if err := m.Up(); err != nil {
		return err
	}

	return nil
}

func (p *PostgresClient) MigrateDown(dbName string) error {
	m, err := p.prepareConnection(dbName)
	if err != nil {
		return err
	}

	if err := m.Down(); err != nil {
		return err
	}

	return nil
}

func (p *PostgresClient) prepareConnection(dbName string) (*migrate.Migrate, error) {
	conn, err := p.db.DB()
	if err != nil {
		return nil, err
	}

	driver, err := migratePsql.WithInstance(conn, &migratePsql.Config{})
	if err != nil {
		return nil, err
	}

	m, err := migrate.NewWithDatabaseInstance("file://migrations/postgres", dbName, driver)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create migrations instance")
	}
	return m, nil
}
