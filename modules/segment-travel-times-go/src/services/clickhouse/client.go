package services

import (
	"context"
	"fmt"
	"main/src/lib"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
)

// ClickhouseService wraps the ClickHouse connection and provides methods for database operations.
type ClickhouseService struct {
	conn clickhouse.Conn
}

// ClickhouseClientParams contains the configuration for connecting to ClickHouse.
type ClickhouseClientParams struct {
	Host     string
	Port     int
	Database string
	Username string
	Password string
}

// NewClickhouseClient creates a new ClickHouse client and returns a ClickhouseService.
func NewClickhouseClient(options ClickhouseClientParams) (*ClickhouseService, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{fmt.Sprintf("%s:%d", options.Host, options.Port)},
		Auth: clickhouse.Auth{
			Database: options.Database,
			Username: options.Username,
			Password: options.Password,
		},
		Settings: clickhouse.Settings{
			"max_execution_time": 60,
		},
		DialTimeout:     30 * time.Second,
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: time.Hour,
	})
	if err != nil {
		return nil, lib.AppLogger.Error("failed to connect to ClickHouse", err.Error())
	}

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := conn.Ping(ctx); err != nil {
		return nil, lib.AppLogger.Error("failed to ping ClickHouse", err.Error())
	}

	lib.AppLogger.Info("Successfully connected to ClickHouse!")

	return &ClickhouseService{conn: conn}, nil
}

// Close closes the ClickHouse connection.
func (s *ClickhouseService) Close() error {
	return s.conn.Close()
}

// Conn returns the underlying ClickHouse connection for advanced operations.
func (s *ClickhouseService) Conn() clickhouse.Conn {
	return s.conn
}