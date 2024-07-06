package storage

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func (ms *MongoStorage) VectorSearch(queryVector []float32, numCandidates, limit int) ([]Document, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	coll := ms.client.Database(ms.database).Collection(ms.collection)

	pipeline := mongo.Pipeline{
		{{Key: "$vectorSearch", Value: bson.D{
			{Key: "index", Value: "embeddings_index"},
			{Key: "path", Value: "doc_embedding"},
			{Key: "queryVector", Value: queryVector},
			{Key: "numCandidates", Value: numCandidates},
			{Key: "limit", Value: limit},
		}}},
	}

	cursor, err := coll.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []Document
	if err = cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	return results, nil
}
