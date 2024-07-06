package db

import (
	"context"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

var MongoClient *mongo.Client

// InitializeMongoDB initializes the MongoDB client connection.
func InitializeMongoDB(uri string) error {
	// Connect to MongoDB using the default or injected connect function
	var err error
	MongoClient, err = DefaultMongoConnectFunc(context.Background(), uri)
	if err != nil {
		log.Printf("Failed to connect to MongoDB: %v", err)
		return err
	}

	// Adding a timeout to the ping context
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = MongoClient.Ping(ctx, readpref.Primary())
	if err != nil {
		log.Printf("Failed to ping MongoDB: %v", err)
		return err
	}

	log.Println("Connected to MongoDB successfully")
	return nil
}

// DefaultMongoConnectFunc connects to MongoDB with the provided URI.
func DefaultMongoConnectFunc(ctx context.Context, uri string) (*mongo.Client, error) {
	clientOpts := options.Client().ApplyURI(uri)
	client, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		return nil, err
	}

	return client, nil
}

// GetCollection returns a collection from the MongoDB database.
func GetCollection() *mongo.Collection {
	return MongoClient.Database("git_metrics").Collection("metrics")
}
