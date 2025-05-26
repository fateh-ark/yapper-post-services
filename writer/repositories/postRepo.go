package repositories

import (
	"context"
	"errors"
	"time"

	"fateh-ark/yapper-post-writer-service/model"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// PostRepository defines the interface for interacting with the Post collection
type PostRepository interface {
	CreatePost(ctx context.Context, post *model.Post) (*model.Post, error)
	GetPostByID(ctx context.Context, id primitive.ObjectID) (*model.Post, error)
	GetPostsByUserID(ctx context.Context, userID string, limit, skip int64) ([]model.Post, error)
	UpdatePostContent(ctx context.Context, id primitive.ObjectID, content string) error
	IncrementLikeCount(ctx context.Context, id primitive.ObjectID) error
	DecrementLikeCount(ctx context.Context, id primitive.ObjectID) error
	IncrementReplyCount(ctx context.Context, id primitive.ObjectID) error
	DeletePost(ctx context.Context, id primitive.ObjectID) error // Soft delete by setting status
}

type postRepositoryImpl struct {
	collection *mongo.Collection
}

// NewPostRepository creates a new instance of PostRepository
func NewPostRepository(client *mongo.Client, dbName, collectionName string) PostRepository {
	collection := client.Database(dbName).Collection(collectionName)
	return &postRepositoryImpl{collection: collection}
}

// CreatePost inserts a new post into the database.
func (r *postRepositoryImpl) CreatePost(ctx context.Context, post *model.Post) (*model.Post, error) {
	if post.ID.IsZero() {
		post.ID = primitive.NewObjectID()
	}
	if post.Timestamp.IsZero() {
		post.Timestamp = time.Now()
	}
	if post.Status == "" {
		post.Status = "active"
	}

	_, err := r.collection.InsertOne(ctx, post)
	if err != nil {
		return nil, err
	}
	return post, nil
}

// GetPostByID retrieves a post by its ID.
func (r *postRepositoryImpl) GetPostByID(ctx context.Context, id primitive.ObjectID) (*model.Post, error) {
	var post model.Post
	filter := bson.M{"_id": id, "status": "active"} // Only retrieve active posts
	err := r.collection.FindOne(ctx, filter).Decode(&post)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, errors.New("post not found")
		}
		return nil, err
	}
	return &post, nil
}

// GetPostsByUserID retrieves posts by a user ID, with pagination.
func (r *postRepositoryImpl) GetPostsByUserID(ctx context.Context, userID string, limit, skip int64) ([]model.Post, error) {
	var posts []model.Post
	filter := bson.M{"userId": userID, "status": "active"} // Only retrieve active posts

	findOptions := options.Find()
	findOptions.SetLimit(limit)
	findOptions.SetSkip(skip)
	findOptions.SetSort(bson.D{{Key: "timestamp", Value: -1}}) // Sort by most recent

	cursor, err := r.collection.Find(ctx, filter, findOptions)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	if err = cursor.All(ctx, &posts); err != nil {
		return nil, err
	}
	return posts, nil
}

// UpdatePostContent updates the content of an existing post.
func (r *postRepositoryImpl) UpdatePostContent(ctx context.Context, id primitive.ObjectID, content string) error {
	filter := bson.M{"_id": id, "status": "active"}
	update := bson.M{"$set": bson.M{"content": content, "timestamp": time.Now()}} // Also update timestamp

	res, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return errors.New("post not found or not active")
	}
	return nil
}

// IncrementLikeCount increments the likeCount for a post.
func (r *postRepositoryImpl) IncrementLikeCount(ctx context.Context, id primitive.ObjectID) error {
	filter := bson.M{"_id": id, "status": "active"}
	update := bson.M{"$inc": bson.M{"likeCount": 1}}

	res, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return errors.New("post not found or not active")
	}
	return nil
}

// DecrementLikeCount decrements the likeCount for a post.
func (r *postRepositoryImpl) DecrementLikeCount(ctx context.Context, id primitive.ObjectID) error {
	filter := bson.M{"_id": id, "status": "active", "likeCount": bson.M{"$gt": 0}} // Only decrement if > 0
	update := bson.M{"$inc": bson.M{"likeCount": -1}}

	res, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		// This could mean post not found, not active, or likeCount was already 0
		return errors.New("post not found, not active, or likeCount already zero")
	}
	return nil
}

// IncrementReplyCount increments the replyCount for a post.
func (r *postRepositoryImpl) IncrementReplyCount(ctx context.Context, id primitive.ObjectID) error {
	filter := bson.M{"_id": id, "status": "active"}
	update := bson.M{"$inc": bson.M{"replyCount": 1}}

	res, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return errors.New("post not found or not active")
	}
	return nil
}

// DeletePost performs a soft delete by setting the post's status to 'deleted'.
func (r *postRepositoryImpl) DeletePost(ctx context.Context, id primitive.ObjectID) error {
	filter := bson.M{"_id": id, "status": "active"}
	update := bson.M{"$set": bson.M{"status": "deleted"}}

	res, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return errors.New("post not found or already deleted")
	}
	return nil
}
