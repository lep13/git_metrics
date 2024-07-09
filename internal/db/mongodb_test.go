package db

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

// MockMongoClient simulates the MongoDB client behavior.
type MockMongoClient struct {
	PingFunc     func(ctx context.Context, rp *readpref.ReadPref) error
	DatabaseFunc func(name string, opts ...*options.DatabaseOptions) *mongo.Database
}

func (m *MockMongoClient) Ping(ctx context.Context, rp *readpref.ReadPref) error {
	if m.PingFunc != nil {
		return m.PingFunc(ctx, rp)
	}
	return nil
}

func (m *MockMongoClient) Database(name string, opts ...*options.DatabaseOptions) *mongo.Database {
	if m.DatabaseFunc != nil {
		return m.DatabaseFunc(name, opts...)
	}
	return &mongo.Database{}
}

func TestInitializeMongoDB_Success(t *testing.T) {
	// Mock Mongo connect function
	originalMongoConnectFunc := DefaultMongoConnectFunc
	defer func() { DefaultMongoConnectFunc = originalMongoConnectFunc }() // Restore original after test
	DefaultMongoConnectFunc = func(ctx context.Context, uri string) (MongoClientInterface, error) {
		return &MockMongoClient{}, nil // Mock success
	}

	err := InitializeMongoDB("mongodb://mock-uri")
	assert.NoError(t, err)
	assert.NotNil(t, MongoClient)
}

func TestInitializeMongoDB_Failure(t *testing.T) {
	// Mock Mongo connect function
	originalMongoConnectFunc := DefaultMongoConnectFunc
	defer func() { DefaultMongoConnectFunc = originalMongoConnectFunc }() // Restore original after test
	DefaultMongoConnectFunc = func(ctx context.Context, uri string) (MongoClientInterface, error) {
		return nil, errors.New("failed to connect to MongoDB")
	}

	err := InitializeMongoDB("mongodb://invalid-uri")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect to MongoDB")
}

func TestPingMongoDB_Success(t *testing.T) {
	clientOpts := options.Client().ApplyURI("mongodb://localhost:27017")
	client, err := mongo.Connect(context.Background(), clientOpts)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	MongoClient = &MongoClientWrapper{Client: client}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = MongoClient.Ping(ctx, readpref.Primary())
	if err != nil {
		t.Skipf("MongoDB server is not running. Skipping test. Error: %v", err)
	}
}

func TestInitializeMongoDB_MongoPingError(t *testing.T) {
	// Mock Mongo connect function
	originalMongoConnectFunc := DefaultMongoConnectFunc
	defer func() { DefaultMongoConnectFunc = originalMongoConnectFunc }() // Restore original after test
	DefaultMongoConnectFunc = func(ctx context.Context, uri string) (MongoClientInterface, error) {
		return &MockMongoClient{
			PingFunc: func(ctx context.Context, rp *readpref.ReadPref) error {
				return errors.New("failed to ping MongoDB")
			},
		}, nil
	}

	err := InitializeMongoDB("mongodb://mock-uri")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to ping MongoDB")
}

func TestMongoClientWrapper_Database(t *testing.T) {
	// Create a mock Mongo client and wrap it in MongoClientWrapper
	mockMongoClient := &mongo.Client{}
	wrapper := &MongoClientWrapper{Client: mockMongoClient}

	// Mock the Database function to return a basic mock database
	dbName := "testdb"
	mockMongoClient.Database(dbName)

	// Call Database on the wrapper and check the returned value
	db := wrapper.Database(dbName)
	assert.NotNil(t, db)
	assert.Equal(t, dbName, db.Name())
}

func TestDefaultMongoConnectFunc_Success(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := DefaultMongoConnectFunc(ctx, "mongodb://mock-uri")
	assert.NoError(t, err)
	assert.NotNil(t, client)
}

func TestDefaultMongoConnectFunc_Failure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := DefaultMongoConnectFunc(ctx, "invalid-uri")
	assert.Error(t, err)
}

// func TestGetCollection(t *testing.T) {
// 	mockMongoClient := &MockMongoClient{
// 		DatabaseFunc: func(name string, opts ...*options.DatabaseOptions) *mongo.Database {
// 			return &mongo.Database{}
// 		},
// 	}
// 	MongoClient = mockMongoClient

// 	collection := GetCollection()
// 	assert.NotNil(t, collection)
// 	assert.Equal(t, "git_metrics", collection.Name())
// }
