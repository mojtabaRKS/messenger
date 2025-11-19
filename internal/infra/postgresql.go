package infra

import (
	"arvan/message-gateway/intrernal/config"
	"context"
	"fmt"
	"github.com/pkg/errors"

	"github.com/golang-migrate/migrate/v4"
	migratePsql "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
	log "github.com/sirupsen/logrus"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func NewPostgresClient(ctx context.Context, cfg config.Postgres) (*gorm.DB, error) {
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

	return db.WithContext(ctx), nil
}

func MigrateUp(gormDB *gorm.DB, dbName string, logger *log.Logger) error {
	m, err := prepareConnection(gormDB, dbName, logger)
	if err != nil {
		return err
	}

	if err := m.Up(); err != nil {
		return err
	}

	logger.Info("migrations applied successfully")
	return nil
}

func MigrateDown(gormDB *gorm.DB, dbName string, logger *log.Logger) error {
	m, err := prepareConnection(gormDB, dbName, logger)
	if err != nil {
		return err
	}

	if err := m.Down(); err != nil {
		return err
	}

	logger.Info("migrations applied successfully")
	return nil
}

func prepareConnection(gormDB *gorm.DB, dbName string, logger *log.Logger) (*migrate.Migrate, error) {
	conn, err := gormDB.DB()
	if err != nil {
		return nil, err
	}

	driver, err := migratePsql.WithInstance(conn, &migratePsql.Config{})
	if err != nil {
		return nil, err
	}

	logger.Info("migrations started")

	m, err := migrate.NewWithDatabaseInstance("file://migrations", dbName, driver)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create migrations instance")
	}
	return m, nil
}
