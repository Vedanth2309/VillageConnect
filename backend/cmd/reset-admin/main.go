package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"

	"villageconnect/internal/config"
)

func main() {
	// Load environment variables from backend/.env
	if err := godotenv.Load("../.env"); err != nil {
		log.Fatal("Could not load .env. Run this command from the backend directory.")
	}

	adminEmail := os.Getenv("ADMIN_EMAIL")
	if adminEmail == "" {
		adminEmail = "admin@villageconnect.local"
	}

	password := os.Getenv("ADMIN_RESET_PASSWORD")
	if len(password) < 12 || len(password) > 72 {
		log.Fatal("Set ADMIN_RESET_PASSWORD to a password of 12–72 bytes.")
	}

	cfg := config.Load()

	ctx, cancel := context.WithTimeout(
		context.Background(), 15*time.Second,
	)
	defer cancel()

	client, err := mongo.Connect(
		ctx,
		options.Client().ApplyURI(cfg.MongoURI),
	)
	if err != nil {
		log.Fatal("MongoDB connection failed: ", err)
	}
	defer func() {
		_ = client.Disconnect(context.Background())
	}()

	if err := client.Ping(ctx, nil); err != nil {
		log.Fatal("Cannot reach MongoDB: ", err)
	}

	hash, err := bcrypt.GenerateFromPassword(
		[]byte(password), bcrypt.DefaultCost,
	)
	if err != nil {
		log.Fatal("Password hashing failed: ", err)
	}

	collection := client.Database(cfg.MongoDatabase).
		Collection("users")

	filter := bson.M{"email": adminEmail}

	update := bson.M{
		"$set": bson.M{
			"name":         "VillageConnect Admin",
			"email":        adminEmail,
			"role":         "admin",
			"verified":     true,
			"active":       true,
			"passwordHash": string(hash),
		},
		"$setOnInsert": bson.M{
			"_id":       "u4",
			"phone":     "9090909090",
			"villageId": "v1",
		},
	}

	result, err := collection.UpdateOne(
		ctx,
		filter,
		update,
		options.Update().SetUpsert(true),
	)
	if err != nil {
		log.Fatal("Admin reset failed: ", err)
	}

	if result.UpsertedCount == 1 {
		fmt.Println("Created the admin account.")
	} else {
		fmt.Println("Updated the existing admin account.")
	}

	fmt.Println("Admin email:", adminEmail)
	fmt.Println("Admin password reset successfully.")
}
