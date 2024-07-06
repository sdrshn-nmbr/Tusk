package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"strconv"
	"time"

	"github.com/sdrshn-nmbr/tusk/internal/ai"
	"github.com/sdrshn-nmbr/tusk/internal/config"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoStorage struct {
	client     *mongo.Client
	database   string
	collection string
}

type Document struct {
    ID           primitive.ObjectID `bson:"_id,omitempty"`
    Filename     string             `bson:"filename"`
    Content      primitive.Binary   `bson:"content"`
    Chunks       []string           `bson:"chunks"`
    DocEmbedding []float32          `bson:"doc_embedding"`
    Metadata     map[string]string  `bson:"metadata,omitempty"`
}

func NewMongoStorage(cfg *config.Config, collection string) (*MongoStorage, error) {
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(cfg.MongoDBURI))
	if err != nil {
		return nil, err
	}

	return &MongoStorage{
		client:     client,
		database:   cfg.MongoDBDatabase,
		collection: collection,
	}, nil

}

func (ms *MongoStorage) SaveFile(filename string, content io.Reader, embedder *ai.Embedder) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	data, err := io.ReadAll(content)
	if err != nil {
		log.Printf("Error reading file content: %v", err)
		return err
	}

	// Check if the file is a PDF
	if !isPDF(data) {
		log.Printf("File is not a PDF: %s", filename)
		return fmt.Errorf("file is not a PDF")
	}

	text, err := extractTextFromPDF(bytes.NewReader(data))
	if err != nil {
		log.Printf("Error extracting text from PDF: %v", err)
		text = "Text extraction failed"
	}

	chunks := chunkText(text, 1000) // Adjust chunk size as needed

	// Generate embedding for the entire text
	embedding, err := embedder.GenerateEmbedding(text)
	if err != nil {
		log.Printf("Error generating embedding: %v", err)
		return err
	}

	doc := Document{
		Filename: filename,
		Content: primitive.Binary{
			Subtype: 0x00,
			Data:    data,
		},
		Chunks:       chunks,
		DocEmbedding: embedding,
		Metadata: map[string]string{
			"uploadDate": time.Now().Format(time.RFC3339),
			"size":       fmt.Sprintf("%d", len(data)),
		},
	}

	coll := ms.client.Database(ms.database).Collection(ms.collection)
	_, err = coll.InsertOne(ctx, doc)
	if err != nil {
		log.Printf("Error inserting document into MongoDB: %v", err)
		return err
	}

	return nil
}

func isPDF(data []byte) bool {
	// Check for PDF magic number
	return len(data) > 4 && string(data[:4]) == "%PDF"
}

func (ms *MongoStorage) GetFile(filename string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var result Document
	collection := ms.client.Database(ms.database).Collection(ms.collection)
	err := collection.FindOne(ctx, bson.M{"filename": filename}).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("file not found")
		}
		return nil, err
	}

	return result.Content.Data, nil

}

func (ms *MongoStorage) DeleteFileFunc(filename string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	collection := ms.client.Database(ms.database).Collection(ms.collection)
	_, err := collection.DeleteOne(ctx, bson.M{"filename": filename})

	if err != nil {
		if err == mongo.ErrNoDocuments {
			return errors.New("file not found")
		}
		return err
	}

	return nil

}

func (ms *MongoStorage) ListFiles() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	collection := ms.client.Database(ms.database).Collection(ms.collection)
	cursor, err := collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}

	defer cursor.Close(ctx)

	var files []string
	for cursor.Next(ctx) {
		var doc Document
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}

		files = append(files, doc.Filename)
	}

	return files, nil
}

func (ms *MongoStorage) GetFileSize(filename string) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var result Document
	coll := ms.client.Database(ms.database).Collection(ms.collection)
	err := coll.FindOne(ctx, bson.M{"filename": filename}).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return 0, errors.New("file not found")
		}
		return 0, err
	}

	sizeStr, ok := result.Metadata["size"]
	if !ok {
		return 0, errors.New("file size metadata not found")
	}

	size, err := strconv.ParseInt(sizeStr, 10, 64)
	if err != nil {
		return 0, errors.New("invalid file size format in metadata")
	}

	return size, nil
}
