package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var client *mongo.Client
var collection *mongo.Collection
var kafkaProducer *kafka.Producer

// struct
type User struct {
	Username string `json:"username" bson:"username"`
	Password string `json:"password" bson:"password"`
}

func main() {

	// MongoDB setup

	user := os.Getenv("MONGO_USER")
	pass := os.Getenv("MONGO_PASS")
	host := os.Getenv("MONGO_HOST")
	db := os.Getenv("MONGO_DATABASE")
	db_collection := os.Getenv("MONGO_COLLECTION")

	dbURI := fmt.Sprintf("mongodb://%s:%s@%s/", user, pass, host)

	var err error
	client, err = mongo.Connect(context.TODO(), options.Client().ApplyURI(dbURI))
	if err != nil {
		log.Fatal("Error connecting to MongoDB:", err)
	}

	// Initialize MongoDB collection (only once)
	collection = client.Database(db).Collection(db_collection)

	// Kafka producer setup
	kafkaProducer, err = kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers": os.Getenv("KAFKA_BROKER"),
	})
	if err != nil {
		log.Fatalf("Failed to create Kafka producer: %v", err)
	}
	defer kafkaProducer.Close()

	// Gin setup
	r := gin.Default()
	r.POST("/login", loginUser)
	r.POST("/logout", logoutUser)

	// Start the server
	log.Println("login service started on port 8082")
	r.Run(":8081")
}

// Register a new user
func loginUser(c *gin.Context) {
	var user User
	if err := c.BindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	// Check if user exists in MongoDB
	var foundUser User
	err := collection.FindOne(context.TODO(), bson.M{"username": user.Username}).Decode(&foundUser)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error retrieving user"})
		}
		return
	}

	// Compare the passwords directly (no hashing)
	if foundUser.Password != user.Password {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	// Log the event to Kafka
	logEvent("login-service", "user_loged_in", user.Username)

	c.JSON(http.StatusCreated, gin.H{"message": "User logged in successfully"})
}

// Register a new user
func logoutUser(c *gin.Context) {
	var user User
	if err := c.BindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	// Check if user exists in MongoDB
	var foundUser User
	err := collection.FindOne(context.TODO(), bson.M{"username": user.Username}).Decode(&foundUser)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error retrieving user"})
		}
		return
	}

	// Compare the passwords directly (no hashing)
	if foundUser.Password != user.Password {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	// Log the event to Kafka
	logEvent("logout-service", "user_loged_out", user.Username)

	c.JSON(http.StatusCreated, gin.H{"message": "User logged out successfully"})
}

// Log events to Kafka
func logEvent(service, event, username string) {

	timestamp := time.Now().Format(time.RFC3339)

	logMessage := map[string]interface{}{
		"service":   service,
		"event":     event,
		"username":  username,
		"timestamp": timestamp,
	}

	message, _ := json.Marshal(logMessage)

	topic := "user-logs"

	msg := &kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Value:          message,
	}

	err := kafkaProducer.Produce(msg, nil)
	if err != nil {
		log.Printf("Failed to write log to Kafka: %v", err)
	}

}
