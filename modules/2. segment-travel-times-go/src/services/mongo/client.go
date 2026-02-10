package services

import (
	"context"
	"main/src/lib"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoClient struct {
	client *mongo.Client
	database *mongo.Database
}

// CreateMongoClient creates a new MongoDB client
func NewMongoClient(uri string, database string) (*MongoClient, error) {
	// Set timeout for the connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Set client options
	clientOptions := options.Client().ApplyURI(uri)

	// Create a new MongoDB client
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, lib.AppLogger.Error(err, "failed to connect to MongoDB")
	}

	// Ping the database to verify connection
	if err := client.Ping(ctx, nil); err != nil {
		return nil, lib.AppLogger.Error(err, "failed to ping MongoDB")
	}

	lib.AppLogger.Info("Successfully connected to MongoDB!")

	return &MongoClient{ client: client, database: client.Database(database) }, nil
}

func (s *MongoClient) Close(ctx context.Context) error {
	return s.client.Disconnect(ctx)
}