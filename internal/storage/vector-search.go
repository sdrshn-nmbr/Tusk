package storage

import (
	"context"
	"time"
	"log"

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

    log.Printf("Executing pipeline: %+v", pipeline)

    cursor, err := coll.Aggregate(ctx, pipeline)
    if err != nil {
        log.Printf("Aggregation error: %v", err)
        return nil, err
    }
    defer cursor.Close(ctx)

    var results []Document
    if err = cursor.All(ctx, &results); err != nil {
        log.Printf("Cursor decoding error: %v", err)
        return nil, err
    }

    log.Printf("Number of results: %d", len(results))

    return results, nil
}