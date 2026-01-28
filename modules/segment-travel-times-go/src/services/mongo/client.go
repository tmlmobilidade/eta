package services

import (
	"context"
	"main/src/lib"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoService struct {
	client *mongo.Client
	database *mongo.Database
}

// CreateMongoClient creates a new MongoDB client
func NewMongoClient(uri string) (*MongoService, error) {
	// Set timeout for the connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Set client options
	clientOptions := options.Client().ApplyURI(uri)

	// Create a new MongoDB client
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, lib.AppLogger.Error("failed to connect to MongoDB", err.Error())
	}

	// Ping the database to verify connection
	if err := client.Ping(ctx, nil); err != nil {
		return nil, lib.AppLogger.Error("failed to ping MongoDB", err.Error())
	}

	lib.AppLogger.Info("Successfully connected to MongoDB!")

	return &MongoService{ client: client, database: client.Database("production") }, nil
}

func (s *MongoService) Close(ctx context.Context) error {
	return s.client.Disconnect(ctx)
}