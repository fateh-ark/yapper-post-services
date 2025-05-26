package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Post represents a post document in MongoDB
type Post struct {
	ID         primitive.ObjectID  `bson:"_id,omitempty"` // MongoDB's default primary key
	UserID     string              `bson:"userId"`        // Assuming userId comes from an external user service
	Username   string              `bson:"username"`
	ParentID   *primitive.ObjectID `bson:"parentId,omitempty"` // Nullable: used for replies to other posts
	Content    string              `bson:"content"`
	Timestamp  time.Time           `bson:"timestamp"`
	LikeCount  int                 `bson:"likeCount"`
	ReplyCount int                 `bson:"replyCount"`
	Status     string              `bson:"status"` // e.g., "active", "deleted"
}

// Like represents a like document in MongoDB
type Like struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"` // MongoDB's default primary key
	PostID    primitive.ObjectID `bson:"postId"`
	UserID    string             `bson:"userId"` // Assuming userId comes from an external user service
	Timestamp time.Time          `bson:"timestamp"`
}
