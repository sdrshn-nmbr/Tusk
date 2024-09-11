package storage

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	// "go.mongodb.org/mongo-driver/mongo"
)

func (ms *MongoStorage) MigrateMissingFileSizes() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	coll := ms.client.Database(ms.database).Collection(ms.documentsCollection)
	
	cursor, err := coll.Find(ctx, bson.M{
		"$or": []bson.M{
			{"metadata.size": bson.M{"$exists": false}},
			{"metadata": bson.M{"$exists": false}},
		},
	})
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var doc Document
		if err := cursor.Decode(&doc); err != nil {
			log.Printf("Error decoding document: %v", err)
			continue
		}

		size := len(doc.Content.Data)
		if doc.Metadata == nil {
			doc.Metadata = make(map[string]string)
		}
		doc.Metadata["size"] = fmt.Sprintf("%d", size)

		_, err := coll.UpdateOne(
			ctx,
			bson.M{"_id": doc.ID},
			bson.M{"$set": bson.M{"metadata": doc.Metadata}},
		)
		if err != nil {
			log.Printf("Error updating document %s: %v", doc.Filename, err)
		}
	}

	return nil
}