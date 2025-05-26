package main

import (
	"context"
	"log"
	"os"
	"time"

	"fateh-ark/yapper-post-writer-service/model"        // Adjust import path based on your module name
	"fateh-ark/yapper-post-writer-service/repositories" // Adjust import path

	"go.mongodb.org/mongo-driver/bson/primitive" // Added for ObjectID if you want to use it in main for tests
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

func main() {
	// Load environment variables for MongoDB connection
	mongoURI := os.Getenv("MONGO_URI")
	mongoDBName := os.Getenv("MONGO_DB_NAME")
	if mongoURI == "" || mongoDBName == "" {
		log.Fatal("MONGO_URI and MONGO_DB_NAME environment variables must be set")
	}

	// Set up context for DB connection with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second) // Increased timeout for initial connection
	defer cancel()

	// Connect to MongoDB
	// Use `SetDirect(true)` if you only have one host in URI and face issues,
	// but for replica sets, connecting to any member usually discovers the others.
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer func() {
		// Ensure the MongoDB client disconnects when main exits
		if err = client.Disconnect(ctx); err != nil {
			log.Fatalf("Error disconnecting from MongoDB: %v", err)
		}
		log.Println("Disconnected from MongoDB.")
	}()

	// Ping the primary to verify connection and replica set health
	// readpref.Primary() ensures it pings the current primary
	err = client.Ping(ctx, readpref.Primary())
	if err != nil {
		log.Fatalf("Failed to ping MongoDB primary. Is the replica set initiated and healthy? Error: %v", err)
	}
	log.Println("Successfully connected and pinged MongoDB primary!")

	// Initialize repositories
	postRepo := repositories.NewPostRepository(client, mongoDBName, "posts")
	likeRepo := repositories.NewLikeRepository(client, mongoDBName, "likes")

	// Ensure all necessary indexes are created for the collections.
	// This is critical for performance and enforcing unique constraints (like on likes).
	log.Println("Ensuring indexes for Post collection (if any)...")
	// If you had indexes for Post, you'd call a similar method here, e.g.:
	// if err = postRepo.(*repositories.postRepositoryImpl).EnsureIndexes(ctx); err != nil {
	// 	log.Fatalf("Failed to ensure Post collection indexes: %v", err)
	// }
	log.Println("Ensuring indexes for Like collection...")
	if err = likeRepo.EnsureIndexes(ctx); err != nil { // Type assertion to access private method
		log.Fatalf("Failed to ensure Like collection indexes: %v", err)
	}
	log.Println("Indexes for Like collection ensured.")

	// --- Example Usage of Repositories (for testing/demonstration) ---
	// You would typically move this logic into your service layer.
	// For now, it's here to show how to interact with the repositories.

	// Create a new post
	newPost := &model.Post{
		UserID:   "user_test_123",
		Username: "testuser_writer",
		Content:  "Hello, this is my first post from Go writer service!",
	}
	createdPost, err := postRepo.CreatePost(context.Background(), newPost)
	if err != nil {
		log.Printf("Error creating post: %v", err)
	} else {
		log.Printf("Created post: ID=%s, Content='%s'", createdPost.ID.Hex(), createdPost.Content)
	}

	// Let's assume we have a post ID to work with from the creation above
	var postId primitive.ObjectID
	if createdPost != nil {
		postId = createdPost.ID
	} else {
		// If post creation failed, try to get an existing post for like testing
		// In a real app, you'd handle this more gracefully.
		log.Println("Could not create post, trying to find an existing one for like operations...")
		// This is just a placeholder, you'd need a real way to get an ID
		// postId, _ = primitive.ObjectIDFromHex("some_existing_post_id_hex")
		// log.Println("Using placeholder post ID for like operations.")
		log.Println("Skipping like operations as no post ID is available.")
		postId = primitive.NilObjectID // Set to nil to prevent further errors if no post ID
	}

	if postId != primitive.NilObjectID {
		// Like the post
		newLike := &model.Like{
			PostID: postId,
			UserID: "liking_user_A",
		}
		_, err = likeRepo.CreateLike(context.Background(), newLike)
		if err != nil {
			log.Printf("Error creating like (user A): %v", err)
		} else {
			log.Println("User A liked the post.")
			// Increment post like count (business logic would typically be in service layer)
			postRepo.IncrementLikeCount(context.Background(), postId)
		}

		// Try to like again with same user (should fail due to unique index)
		_, err = likeRepo.CreateLike(context.Background(), newLike)
		if err != nil {
			log.Printf("Expected error for duplicate like (user A): %v", err)
		}

		// Another user likes the post
		newLikeB := &model.Like{
			PostID: postId,
			UserID: "liking_user_B",
		}
		_, err = likeRepo.CreateLike(context.Background(), newLikeB)
		if err != nil {
			log.Printf("Error creating like (user B): %v", err)
		} else {
			log.Println("User B liked the post.")
			postRepo.IncrementLikeCount(context.Background(), postId)
		}

		// Get all likes for this post
		likesForPost, err := likeRepo.GetLikesByPostID(context.Background(), postId)
		if err != nil {
			log.Printf("Error getting likes for post %s: %v", postId.Hex(), err)
		} else {
			log.Printf("Likes for post %s (Count: %d):", postId.Hex(), len(likesForPost))
			for _, like := range likesForPost {
				log.Printf("  - Like ID: %s, UserID: %s, Timestamp: %s", like.ID.Hex(), like.UserID, like.Timestamp.Format(time.RFC3339))
			}
		}

		// Get all likes by a specific user
		likesByUserA, err := likeRepo.GetLikesByUserID(context.Background(), "liking_user_A")
		if err != nil {
			log.Printf("Error getting likes by user_A: %v", err)
		} else {
			log.Printf("Likes by user_A (Count: %d):", len(likesByUserA))
			for _, like := range likesByUserA {
				log.Printf("  - PostID: %s, Like ID: %s, Timestamp: %s", like.PostID.Hex(), like.ID.Hex(), like.Timestamp.Format(time.RFC3339))
			}
		}

		// User A unlikes the post
		err = likeRepo.DeleteLike(context.Background(), postId, "liking_user_A")
		if err != nil {
			log.Printf("Error unliking post (user A): %v", err)
		} else {
			log.Println("User A unliked the post.")
			postRepo.DecrementLikeCount(context.Background(), postId) // Decrement count
		}

		// Verify user A's like is gone
		hasLiked, err := likeRepo.HasUserLikedPost(context.Background(), postId, "liking_user_A")
		if err != nil {
			log.Printf("Error checking if user A liked post: %v", err)
		} else {
			log.Printf("Has user A liked post after unliking? %t", hasLiked)
		}

		// Fetch updated post to see like count
		updatedPost, err := postRepo.GetPostByID(context.Background(), postId)
		if err != nil {
			log.Printf("Error getting updated post: %v", err)
		} else {
			log.Printf("Updated post like count: %d", updatedPost.LikeCount)
		}
	}

	// --- Start your HTTP/gRPC server here ---
	// This is where your service would typically listen for incoming requests.
	log.Println("Writer service is running and ready to handle requests...")

	// Keep the main goroutine alive indefinitely to keep the service running
	select {}
}
