package repositories

import (
	"context"
	"errors"
	"time"

	"fateh-ark/yapper-post-writer-service/model" // Adjust import path based on your module name

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// LikeRepository defines the interface for interacting with the Like collection
type LikeRepository interface {
	CreateLike(ctx context.Context, like *model.Like) (*model.Like, error)
	DeleteLike(ctx context.Context, postID primitive.ObjectID, userID string) error
	HasUserLikedPost(ctx context.Context, postID primitive.ObjectID, userID string) (bool, error)
	GetLikesByPostID(ctx context.Context, postID primitive.ObjectID) ([]model.Like, error)
	GetLikesByUserID(ctx context.Context, userID string) ([]model.Like, error)
	EnsureIndexes(ctx context.Context) error
}

type likeRepositoryImpl struct {
	collection *mongo.Collection
}

// NewLikeRepository creates a new instance of LikeRepository
func NewLikeRepository(client *mongo.Client, dbName, collectionName string) LikeRepository {
	collection := client.Database(dbName).Collection(collectionName)
	return &likeRepositoryImpl{collection: collection}
}

// CreateLike inserts a new like into the database.
// It relies on a unique index on {postId, userId} to prevent duplicates.
func (r *likeRepositoryImpl) CreateLike(ctx context.Context, like *model.Like) (*model.Like, error) {
	if like.ID.IsZero() {
		like.ID = primitive.NewObjectID()
	}
	if like.Timestamp.IsZero() {
		like.Timestamp = time.Now()
	}

	_, err := r.collection.InsertOne(ctx, like)
	if err != nil {
		// Handle duplicate key error if user already liked the post
		var writeException mongo.WriteException
		if errors.As(err, &writeException) {
			for _, e := range writeException.WriteErrors {
				if e.Code == 11000 { // Duplicate key error code
					return nil, errors.New("user has already liked this post")
				}
			}
		}
		return nil, err
	}
	return like, nil
}

// DeleteLike removes a like from the database.
func (r *likeRepositoryImpl) DeleteLike(ctx context.Context, postID primitive.ObjectID, userID string) error {
	filter := bson.M{"postId": postID, "userId": userID}
	res, err := r.collection.DeleteOne(ctx, filter)
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return errors.New("like not found")
	}
	return nil
}

// HasUserLikedPost checks if a user has already liked a specific post.
func (r *likeRepositoryImpl) HasUserLikedPost(ctx context.Context, postID primitive.ObjectID, userID string) (bool, error) {
	filter := bson.M{"postId": postID, "userId": userID}
	count, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetLikesByPostID retrieves all likes for a given post ID.
func (r *likeRepositoryImpl) GetLikesByPostID(ctx context.Context, postID primitive.ObjectID) ([]model.Like, error) {
	var likes []model.Like
	filter := bson.M{"postId": postID}

	findOptions := options.Find().SetSort(bson.D{{Key: "timestamp", Value: 1}}) // Ascending timestamp

	cursor, err := r.collection.Find(ctx, filter, findOptions)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	if err = cursor.All(ctx, &likes); err != nil {
		return nil, err
	}
	return likes, nil
}

// GetLikesByUserID retrieves all likes made by a specific user ID.
func (r *likeRepositoryImpl) GetLikesByUserID(ctx context.Context, userID string) ([]model.Like, error) {
	var likes []model.Like
	filter := bson.M{"userId": userID}

	findOptions := options.Find().SetSort(bson.D{{Key: "timestamp", Value: -1}}) // Descending timestamp (most recent likes first)

	cursor, err := r.collection.Find(ctx, filter, findOptions)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	if err = cursor.All(ctx, &likes); err != nil {
		return nil, err
	}
	return likes, nil
}

// -----------------------------------------------------------------------------
// MongoDB Indexing for Like Collection (Important for unique constraint)
// You would typically set up indexes during application startup or migration.
// This is an example of how you'd do it.
// -----------------------------------------------------------------------------

// EnsureIndexes ensures all necessary indexes are created for the Like collection.
func (r *likeRepositoryImpl) EnsureIndexes(ctx context.Context) error {
	// Create unique index on postId and userId to prevent duplicate likes
	indexModel := mongo.IndexModel{
		Keys:    bson.D{{Key: "postId", Value: 1}, {Key: "userId", Value: 1}},
		Options: options.Index().SetUnique(true),
	}

	_, err := r.collection.Indexes().CreateOne(ctx, indexModel)
	if err != nil {
		return errors.New("failed to create unique index on postId, userId: " + err.Error())
	}

	// Other indexes from your schema (e.g., single field indexes) are good too
	// For example, if you frequently query by userId on its own:
	// _, err = r.collection.Indexes().CreateOne(ctx, mongo.IndexModel{
	// 	Keys: bson.D{{Key: "userId", Value: 1}},
	// })
	// if err != nil {
	// 	return errors.New("failed to create index on userId: " + err.Error())
	// }
	return nil
}
