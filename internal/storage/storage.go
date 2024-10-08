// storage.go
package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"path/filepath"
	// "runtime"
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
	UserID   string             `bson:"user_id"`
}

type Chunk struct {
	ID         primitive.ObjectID `bson:"_id,omitempty"`
	DocumentID primitive.ObjectID `bson:"document_id"`
	Content    string             `bson:"content"`
	Embedding  []float32          `bson:"embedding"`
	parent     string             `bson:"parent"`
	UserID     string             `bson:"user_id"`
}

const (
	numWorkers = 16
	batchSize  = 500
	maxRetries = 3
)

func NewMongoStorage(cfg *config.Config) (*MongoStorage, error) {
	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI(cfg.MongoURI))
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
func (ms *MongoStorage) SaveFile(filename string, content io.Reader, embedder *ai.Embedder, userID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	data, err := io.ReadAll(content)
	if err != nil {
		log.Printf("Error reading file content: %+v", err)
		return err
	}

	var text string
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".pdf":
		text, err = extractTextFromPDF(bytes.NewReader(data))
	case ".docx":
		text, err = extractTextFromDOCX(bytes.NewReader(data))
	case ".txt":
		text, err = string(data), nil
	case ".jpg", ".jpeg", ".png", ".webp", ".heic", ".heif":
		text, err = extractTextFromImage(data)
	default:
		log.Printf("unsupported file type: %s", ext)
		err = fmt.Errorf("unsupported file type: %s", ext)
	}

	if err != nil {
		log.Printf("Error extracting text from file: %+v", err)
		return err
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
		UserID: userID,
	}

	docsColl := ms.client.Database(ms.database).Collection(ms.documentsCollection)
	result, err := docsColl.InsertOne(ctx, doc)
	if err != nil {
		log.Printf("Error inserting document into MongoDB: %+v", err)
		return err
	}

	documentID := result.InsertedID.(primitive.ObjectID)

	chunks := ChunkText(text)

	resultsChan := make(chan Chunk, len(chunks))
	errorChan := make(chan error, len(chunks))
	var wg sync.WaitGroup

	batchSize := 16           // Adjust based on API limits
	maxConcurrentBatches := 4 // Adjust based on system capabilities and API rate limits
	semaphore := make(chan struct{}, maxConcurrentBatches)

	for i := 0; i < len(chunks); i += batchSize {
		end := i + batchSize
		if end > len(chunks) {
			end = len(chunks)
		}
		batchChunks := chunks[i:end]

		wg.Add(1)
		semaphore <- struct{}{}
		go func(batchChunks []string) {
			defer wg.Done()
			defer func() { <-semaphore }()

			var embeddings [][]float32
			var err error
			for retry := 0; retry < maxRetries; retry++ {
				embeddings, err = embedder.GenerateEmbeddings(batchChunks)
				if err == nil {
					break
				}
				time.Sleep(time.Duration(retry*100) * time.Millisecond)
			}
			if err != nil {
				errorChan <- fmt.Errorf("Error generating embeddings: %+v", err)
				return
			}
			if len(embeddings) != len(batchChunks) {
				errorChan <- fmt.Errorf("Mismatch in embeddings count: expected %d, got %d", len(batchChunks), len(embeddings))
				return
			}

			for i, embedding := range embeddings {
				chunk := Chunk{
					DocumentID: documentID,
					Content:    batchChunks[i],
					Embedding:  embedding,
					parent:     filename,
					UserID:     userID,
				}
				resultsChan <- chunk
			}
		}(batchChunks)
	}

	// Wait for all batches to complete
	go func() {
		wg.Wait()
		close(resultsChan)
		close(errorChan)
	}()

	// Collect results and insert into MongoDB
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

func worker(embedder *ai.Embedder, documentID primitive.ObjectID, filename string, userID string, chunkChan <-chan string, resultsChan chan<- Chunk, errorChan chan<- error, wg *sync.WaitGroup) {
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
			UserID:     userID,
		}

		resultsChan <- chunk
	}
}

func (ms *MongoStorage) GetFile(filename string, userID string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var result Document
	collection := ms.client.Database(ms.database).Collection(ms.documentsCollection)
	err := collection.FindOne(ctx, bson.M{"filename": filename, "user_id": userID}).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("file not found")
		}
		return nil, err
	}

	return result.Content.Data, nil
}

func (ms *MongoStorage) DeleteFileFunc(filename string, userID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	docsColl := ms.client.Database(ms.database).Collection(ms.documentsCollection)

	var doc Document
	err := docsColl.FindOne(ctx, bson.M{"filename": filename, "user_id": userID}).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return errors.New("file not found")
		}
		return err
	}

	_, err = docsColl.DeleteOne(ctx, bson.M{"filename": filename, "user_id": userID})
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

func (ms *MongoStorage) ListFiles(userID string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	collection := ms.client.Database(ms.database).Collection(ms.documentsCollection)
	cursor, err := collection.Find(ctx, bson.M{"user_id": userID})
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

	// Check if metadata exists
	if result.Metadata == nil {
		return 0, errors.New("file metadata not found")
	}

	sizeStr, ok := result.Metadata["size"]
	if !ok {
		// If size is not in metadata, return the length of the content
		return int64(len(result.Content.Data)), nil
	}

	size, err := strconv.ParseInt(sizeStr, 10, 64)
	if err != nil {
		// If we can't parse the size, return the length of the content
		return int64(len(result.Content.Data)), nil
	}

	return size, nil
}
