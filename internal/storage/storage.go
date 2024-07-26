// new storage.go with concurrency
package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sdrshn-nmbr/tusk/internal/ai"
	"github.com/sdrshn-nmbr/tusk/internal/config"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoStorage struct {
	client              *mongo.Client
	database            string
	documentsCollection string
	chunksCollection    string
}

type Document struct {
	ID       primitive.ObjectID `bson:"_id,omitempty"`
	Filename string             `bson:"filename"`
	Content  primitive.Binary   `bson:"content"`
	Metadata map[string]string  `bson:"metadata,omitempty"`
}

type Chunk struct {
	ID         primitive.ObjectID `bson:"_id,omitempty"`
	DocumentID primitive.ObjectID `bson:"document_id"`
	Content    string             `bson:"content"`
	Embedding  []float32          `bson:"embedding"`
	parent     string             `bson:"parent"`
}

const (
	numWorkers = 16
	batchSize  = 500
	maxRetries = 3
)

func NewMongoStorage(cfg *config.Config) (*MongoStorage, error) {
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(cfg.MongoDBURI))
	if err != nil {
		return nil, err
	}

	return &MongoStorage{
		client:              client,
		database:            cfg.MongoDBDatabase,
		documentsCollection: "documents",
		chunksCollection:    "chunks",
	}, nil
}

func (ms *MongoStorage) SaveFile(filename string, content io.Reader, embedder *ai.Embedder) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second) // A bit longer than 10s here to allow for bigger docs to be processed
	defer cancel()

	data, err := io.ReadAll(content)
	if err != nil {
		log.Printf("Error reading file content: %+v", err)
		return err
	}

	// if !isValidFileType(data) {
	// 	log.Printf("File is not a valid format: %s", filename)
	// 	return fmt.Errorf("file is not a valid format")
	// }

	var text string
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".pdf":
		text, err = extractTextFromPDF(bytes.NewReader(data))
	case ".docx":
		text, err = extractTextFromDOCX(bytes.NewReader(data))
	case ".txt":
		text, err = string(data), nil
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp":
		text, err = extractTextFromImage(data)
		log.Print("\n\n\nEXTRACTING TEXT FROM IMG\n\n\n")
	default:
		log.Printf("unsupported file type: %s", ext)
		err = fmt.Errorf("unsupported file type: %s", ext)
	}

	if err != nil {
		log.Printf("Error extracting text from file: %+v", err)
		text = "Text extraction failed"
	}

	doc := Document{
		Filename: filename,
		Content: primitive.Binary{
			Subtype: 0x00,
			Data:    data,
		},
		Metadata: map[string]string{
			"uploadDate": time.Now().Format(time.RFC3339),
			"size":       fmt.Sprintf("%d", len(data)),
		},
	}

	docsColl := ms.client.Database(ms.database).Collection(ms.documentsCollection)
	result, err := docsColl.InsertOne(ctx, doc)
	if err != nil {
		log.Printf("Error inserting document into MongoDB: %+v", err)
		return err
	}

	documentID := result.InsertedID.(primitive.ObjectID)

	chunks := ChunkText(text)

	chunkChan := make(chan string, len(chunks))
	resultsChan := make(chan Chunk, len(chunks))
	errorChan := make(chan error, numWorkers)
	var wg sync.WaitGroup

	// Determine optimal number of workers
	optimalWorkers := runtime.NumCPU() * 2

	if optimalWorkers > numWorkers {
		optimalWorkers = numWorkers
	}

	// Start worker goroutines
	for i := 0; i < optimalWorkers; i++ {
		wg.Add(1)
		go worker(embedder, documentID, filename, chunkChan, resultsChan, errorChan, &wg)
	}

	// Send chunks to workers
	go func() {
		for _, chunk := range chunks {
			chunkChan <- chunk
		}
		close(chunkChan)
	}()

	// Wait for all workers to finish
	go func() {
		wg.Wait()
		close(resultsChan)
		close(errorChan)
	}()

	// Collect results and insert into db
	return ms.insertChunks(ctx, resultsChan, errorChan)
}

func (ms *MongoStorage) insertChunks(ctx context.Context, resultsChan <-chan Chunk, errorChan <-chan error) error {
	var bulkOps []mongo.WriteModel
	chunksColl := ms.client.Database(ms.database).Collection(ms.chunksCollection)

	flushBulkOps := func() error {
		if len(bulkOps) == 0 {
			return nil
		}

		opts := options.BulkWrite().SetOrdered(false)
		_, err := chunksColl.BulkWrite(ctx, bulkOps, opts)
		if err != nil {
			log.Printf("Error performing bulk write operation: %+v", err)
			return err
		}
		bulkOps = bulkOps[:0] // Clear the bulk operations
		return nil
	}

	t01 := time.Now()
	for {
		select {
		case chunk, ok := <-resultsChan:
			if !ok {
				// Channel closed, flush remaining bulk operations
				if err := flushBulkOps(); err != nil {
					return err
				}
				log.Printf("Time taken with workers: %+v", time.Since(t01))
				return nil
			}
			// Create an InsertOne model for each chunk
			insertModel := mongo.NewInsertOneModel().SetDocument(chunk)
			bulkOps = append(bulkOps, insertModel)

			if len(bulkOps) >= batchSize {
				if err := flushBulkOps(); err != nil {
					return err
				}
			}
		case err := <-errorChan:
			if err != nil {
				log.Printf("Error from workers: %+v", err)
				return err
			}
		}
	}
}

func worker(embedder *ai.Embedder, documentID primitive.ObjectID, filename string, chunkChan <-chan string, resultsChan chan<- Chunk, errorChan chan<- error, wg *sync.WaitGroup) {
	defer wg.Done()
	for chunkText := range chunkChan {
		var embedding []float32
		var err error
		for i := 0; i < maxRetries; i++ {
			embedding, err = embedder.GenerateEmbedding(chunkText)
			if err == nil {
				break
			}
			time.Sleep(time.Duration(i*100) * time.Millisecond) // Exponential backoff
		}
		if err != nil {
			errorChan <- fmt.Errorf("failed to generate embedding after %d retries: %+v", maxRetries, err)
			continue
		}

		chunk := Chunk{
			DocumentID: documentID,
			Content:    chunkText,
			Embedding:  embedding,
			parent:     filename,
		}

		resultsChan <- chunk
	}
}

func isValidFileType(data []byte) bool {
	if len(data) < 4 {
		return false
	}

	// Check for PDF magic number
	if string(data[:4]) == "%PDF" {
		return true
	}

	// Check for DOCX (Office Open XML) signature
	// DOCX files are ZIP archives, so they start with PK\x03\x04
	if len(data) >= 4 && data[0] == 0x50 && data[1] == 0x4B && data[2] == 0x03 && data[3] == 0x04 {
		return true
	}

	return false
}

func (ms *MongoStorage) GetFile(filename string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var result Document
	collection := ms.client.Database(ms.database).Collection(ms.documentsCollection)
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

	docsColl := ms.client.Database(ms.database).Collection(ms.documentsCollection)

	var doc Document
	err := docsColl.FindOne(ctx, bson.M{"filename": filename}).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return errors.New("file not found")
		}
		return err
	}

	_, err = docsColl.DeleteOne(ctx, bson.M{"filename": filename})
	if err != nil {
		return err
	}

	chunksColl := ms.client.Database(ms.database).Collection(ms.chunksCollection)
	_, err = chunksColl.DeleteMany(ctx, bson.M{"document_id": doc.ID})
	if err != nil {
		log.Printf("Error deleting chunks: %+v", err)
	}

	return nil
}

func (ms *MongoStorage) ListFiles() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	collection := ms.client.Database(ms.database).Collection(ms.documentsCollection)
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
	coll := ms.client.Database(ms.database).Collection(ms.documentsCollection)
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
