package services

import (
	"context"
	"fmt"
	"main/src/lib"
	"main/src/types"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	driver "github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// ClickhouseClient wraps the ClickHouse connection and provides methods for database operations.
type ClickhouseClient struct {
	conn clickhouse.Conn
}

// NewClickhouseClient creates a new ClickHouse client and returns a ClickhouseClient.
func NewClickhouseClient(options types.ClickhouseClientParams) (*ClickhouseClient, error) {
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
		return nil, lib.AppLogger.Error(err, "failed to connect to ClickHouse")
	}

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := conn.Ping(ctx); err != nil {
		return nil, lib.AppLogger.Error(err, "failed to ping ClickHouse")
	}

	lib.AppLogger.Info("Successfully connected to ClickHouse!")

	return &ClickhouseClient{conn: conn}, nil
}

// Close closes the ClickHouse connection.
func (c *ClickhouseClient) Close() error {
	return c.conn.Close()
}

// Conn returns the underlying ClickHouse connection for advanced operations.
func (c *ClickhouseClient) Conn() clickhouse.Conn {
	return c.conn
}

// QueryAndScan executes a query and scans each row using the provided scanner function.
// Automatically handles row iteration, closing, and error checking.
func (c *ClickhouseClient) QueryAndScan(ctx context.Context, query string, scanner func(rows driver.Rows) error) error {
	rows, err := c.conn.Query(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		if err := scanner(rows); err != nil {
			return lib.AppLogger.Error(err, "failed to scan row")
		}
	}

	if err := rows.Err(); err != nil {
		return lib.AppLogger.Error(err, "error iterating rows")
	}

	return nil
}

// QueryAll executes a query and scans all results into a slice using a generic type.
// The scanner function should scan a single row and return the scanned object.
func QueryAll[T any](c *ClickhouseClient, ctx context.Context, query string, scanner func(rows driver.Rows) (T, error)) ([]T, error) {
	var results []T

	err := c.QueryAndScan(ctx, query, func(rows driver.Rows) error {
		item, err := scanner(rows)
		if err != nil {
			return lib.AppLogger.Error(err, "failed to scan row")
		}
		results = append(results, item)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return results, nil
}