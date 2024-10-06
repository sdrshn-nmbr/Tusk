package database

import (
	"context"
	"log"

	"github.com/sdrshn-nmbr/tusk/internal/config"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoDB struct {
	client   *mongo.Client
	database *mongo.Database
}

func NewMongoDB(cfg *config.Config) (*MongoDB, error) {
	clientOptions := options.Client().ApplyURI(cfg.MongoURI)
	client, err := mongo.Connect(context.TODO(), clientOptions)

	if err != nil {
		return nil, err
	}

	if err := client.Ping(context.TODO(), nil); err != nil {
		return nil, err
	}

	database := client.Database(cfg.MongoDBDatabase)

	return &MongoDB{
		client:   client,
		database: database,
	}, nil
}

func (m *MongoDB) Close() {
	if err := m.client.Disconnect(context.TODO()); err != nil {
		log.Printf("Error disconnecting from MongoDB: %+v", err)
	}
}
