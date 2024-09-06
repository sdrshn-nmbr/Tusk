package storage

import (
	"context"
	// "fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func (ms *MongoStorage) VectorSearch(queryVector []float32, numCandidates, limit int, userID string) ([]Chunk, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	coll := ms.client.Database(ms.database).Collection(ms.chunksCollection)

	pipeline := mongo.Pipeline{
		{{Key: "$vectorSearch", Value: bson.D{
			{Key: "index", Value: "chunks_embedding_index"},
			{Key: "path", Value: "embedding"},
			{Key: "queryVector", Value: queryVector},
			{Key: "numCandidates", Value: numCandidates},
			{Key: "limit", Value: limit},
		}}},
		{{Key: "$match", Value: bson.M{"user_id": userID}}},
		{{Key: "$lookup", Value: bson.D{
			{Key: "from", Value: ms.documentsCollection},
			{Key: "localField", Value: "document_id"},
			{Key: "foreignField", Value: "_id"},
			{Key: "as", Value: "document"},
		}}},
		{{Key: "$unwind", Value: "$document"}},
		{{Key: "$project", Value: bson.D{
			{Key: "content", Value: 1},
			{Key: "embedding", Value: 1},
			{Key: "filename", Value: "$document.filename"},
			{Key: "score", Value: bson.D{{Key: "$meta", Value: "vectorSearchScore"}}},
		}}},
	}

	// log.Printf("Executing pipeline: %+v", pipeline)

	cursor, err := coll.Aggregate(ctx, pipeline)
	if err != nil {
		log.Printf("Aggregation error: %+v", err)
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []Chunk
	if err = cursor.All(ctx, &results); err != nil {
		log.Printf("Cursor decoding error: %+v", err)
		return nil, err
	}

	log.Printf("Number of results: %d", len(results))
	// fmt.Print(results)
	// for _, res := range results {
	// 	fmt.Printf("Content: %s", res.Content)
	// }

	return results, nil
}
