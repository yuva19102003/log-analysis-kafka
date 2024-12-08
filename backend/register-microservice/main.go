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
	r.POST("/register", registerUser)
	r.PUT("/update", updateUser)
	r.DELETE("/delete", deleteUser)

	// Start the server
	log.Println("Register service started on port 8082")
	r.Run(":8082")
}

// Register a new user
func registerUser(c *gin.Context) {
	var user User
	if err := c.BindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	// MongoDB operation to insert a new user
	_, err := collection.InsertOne(context.TODO(), user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	// Log the event to Kafka
	logEvent("register-service", "user_created", user.Username)

	c.JSON(http.StatusCreated, gin.H{"message": "User created successfully"})
}

// Update an existing user
func updateUser(c *gin.Context) {
	var user User
	if err := c.BindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	// MongoDB operation to update an existing user
	filter := bson.M{"username": user.Username}
	update := bson.M{"$set": user}

	_, err := collection.UpdateOne(context.TODO(), filter, update)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
		return
	}

	// Log the event to Kafka
	logEvent("register-service", "user_updated", user.Username)

	c.JSON(http.StatusOK, gin.H{"message": "User updated successfully"})
}

// Delete an existing user
func deleteUser(c *gin.Context) {
	var user User
	if err := c.BindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	// MongoDB operation to delete a user
	_, err := collection.DeleteOne(context.TODO(), bson.M{"username": user.Username})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete user"})
		return
	}

	// Log the event to Kafka
	logEvent("register-service", "user_deleted", user.Username)

	c.JSON(http.StatusOK, gin.H{"message": "User deleted successfully"})
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
