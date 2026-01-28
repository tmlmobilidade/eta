// Package clickhouse provides ClickHouse database operations for segment travel times.
package clickhouse

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// Client wraps a ClickHouse connection with convenience methods.
type Client struct {
	conn driver.Conn
}

// NewClient creates a new ClickHouse client from connection parameters.
func NewClient(host string, port int, database, username, password string) (*Client, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{fmt.Sprintf("%s:%d", host, port)},
		Auth: clickhouse.Auth{
			Database: database,
			Username: username,
			Password: password,
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
		return nil, fmt.Errorf("failed to connect to ClickHouse: %w", err)
	}

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := conn.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping ClickHouse: %w", err)
	}

	return &Client{conn: conn}, nil
}

// NewClientFromURL creates a new ClickHouse client from a connection URL.
func NewClientFromURL(url string) (*Client, error) {
	opts, err := clickhouse.ParseDSN(url)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ClickHouse DSN: %w", err)
	}

	conn, err := clickhouse.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to ClickHouse: %w", err)
	}

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := conn.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping ClickHouse: %w", err)
	}

	return &Client{conn: conn}, nil
}

// Conn returns the underlying ClickHouse connection.
func (c *Client) Conn() driver.Conn {
	return c.conn
}

// Close closes the ClickHouse connection.
func (c *Client) Close() error {
	return c.conn.Close()
}

// Exec executes a query without returning results.
func (c *Client) Exec(ctx context.Context, query string, args ...any) error {
	return c.conn.Exec(ctx, query, args...)
}
