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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second) // ! A bit longer than 10s here to allow for bigger docs to be processed
	defer cancel()

	data, err := io.ReadAll(content)
	if err != nil {
		log.Printf("Error reading file content: %+v", err)
		return err
	}

	if !isPDF(data) {
		log.Printf("File is not a PDF: %s", filename)
		return fmt.Errorf("file is not a PDF")
	}

	text, err := extractTextFromPDF(bytes.NewReader(data))
	if err != nil {
		log.Printf("Error extracting text from PDF: %+v", err)
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

	t0 := time.Now()
	chunks := ChunkText(text, 2048)
	fmt.Printf("\n\n\nTIME TAKEN FOR CHUNK TEXT:      %d", time.Since(t0))

	chunksColl := ms.client.Database(ms.database).Collection(ms.chunksCollection)

	for _, chunkText := range chunks {
		embedding, err := embedder.GenerateEmbedding(chunkText)
		
		// * To see if chunking is working properly
		// log.Printf("\n\n\n<<Chunk Text>>>\n\n\n%s\n", chunkText)

		if err != nil {
			log.Printf("Error generating embedding: %+v", err)
			continue
		}

		chunk := Chunk{
			DocumentID: documentID,
			Content:    chunkText,
			Embedding:  embedding,
			parent:     filename,
		}

		_, err = chunksColl.InsertOne(ctx, chunk)
		if err != nil {
			log.Printf("Error inserting chunk into MongoDB: %+v", err)
		}
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
