// Package mongodb provides MongoDB database operations for rides and shapes.
package mongodb

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Client wraps a MongoDB client with convenience methods.
type Client struct {
	client   *mongo.Client
	database *mongo.Database
}

// NewClient creates a new MongoDB client from a connection URI.
func NewClient(uri, databaseName string) (*Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	clientOptions := options.Client().ApplyURI(uri)
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	// Verify connection
	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	return &Client{
		client:   client,
		database: client.Database(databaseName),
	}, nil
}

// Close closes the MongoDB connection.
func (c *Client) Close(ctx context.Context) error {
	return c.client.Disconnect(ctx)
}

// Collection returns a MongoDB collection.
func (c *Client) Collection(name string) *mongo.Collection {
	return c.database.Collection(name)
}

// Database returns the underlying MongoDB database.
func (c *Client) Database() *mongo.Database {
	return c.database
}
