package db

import (
	"context"
	"log"
	"time"

	"github.com/stretchr/testify/mock"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

// MongoClientInterface defines the interface for MongoDB client methods used in our code.
type MongoClientInterface interface {
	Ping(ctx context.Context, rp *readpref.ReadPref) error
	Database(name string, opts ...*options.DatabaseOptions) *mongo.Database
}

// MongoClientWrapper wraps the actual MongoDB client to conform to our interface.
type MongoClientWrapper struct {
	Client *mongo.Client
}

func (m *MongoClientWrapper) Ping(ctx context.Context, rp *readpref.ReadPref) error {
	return m.Client.Ping(ctx, rp)
}

func (m *MongoClientWrapper) Database(name string, opts ...*options.DatabaseOptions) *mongo.Database {
	return m.Client.Database(name, opts...)
}

// MongoClient holds the actual MongoDB client or a mock for testing.
var MongoClient MongoClientInterface

// MongoConnectFuncType defines the function type for connecting to MongoDB.
type MongoConnectFuncType func(ctx context.Context, uri string) (MongoClientInterface, error)

// DefaultMongoConnectFunc is the default function for connecting to MongoDB.
var DefaultMongoConnectFunc MongoConnectFuncType = func(ctx context.Context, uri string) (MongoClientInterface, error) {
	clientOptions := options.Client().ApplyURI(uri)
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, err
	}
	return &MongoClientWrapper{Client: client}, nil
}

// CollectionGetterFunc is a function type for getting a collection.
type CollectionGetterFunc func() CollectionInterface

// GetCollectionFunc is a package-level variable holding the function to get a collection.
var GetCollectionFunc CollectionGetterFunc = defaultGetCollection

// CollectionInterface defines the methods to be mocked for MongoDB collection.
type CollectionInterface interface {
	UpdateOne(ctx context.Context, filter interface{}, update interface{}, opts ...*options.UpdateOptions) (*mongo.UpdateResult, error)
}

// defaultGetCollection returns the default collection.
func defaultGetCollection() CollectionInterface {
	return MongoClient.Database("dashboard").Collection("git_metrics")
}

// GetCollection returns a collection from the MongoDB database.
func GetCollection() CollectionInterface {
	return GetCollectionFunc()
}

// MockCollection is a mock type for the mongo.Collection used for testing.
type MockCollection struct {
	mock.Mock
}

func (m *MockCollection) UpdateOne(ctx context.Context, filter interface{}, update interface{}, opts ...*options.UpdateOptions) (*mongo.UpdateResult, error) {
	args := m.Called(ctx, filter, update, opts)
	return args.Get(0).(*mongo.UpdateResult), args.Error(1)
}

// MockDatabase is a mock type for the mongo.Database used for testing.
type MockDatabase struct {
	mock.Mock
}

func (m *MockDatabase) Collection(name string, opts ...*options.CollectionOptions) CollectionInterface {
	args := m.Called(name, opts)
	return args.Get(0).(CollectionInterface)
}

// GetMockCollection is a helper function for tests.
func GetMockCollection() *MockCollection {
	return &MockCollection{}
}

// GetMockDatabase is a helper function for tests.
func GetMockDatabase() *MockDatabase {
	return &MockDatabase{}
}

// InitializeMongoDB initializes the MongoDB client connection.
func InitializeMongoDB(uri string) error {
	var err error
	MongoClient, err = DefaultMongoConnectFunc(context.Background(), uri)
	if err != nil {
		log.Printf("Failed to connect to MongoDB: %v", err)
		return err
	}

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
