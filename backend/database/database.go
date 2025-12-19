package database

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Recept struct with added BSON tags
type Recept struct {
	// 'bson:"_id,omitempty"' tells Mongo to use this as the primary key.
	// If it is empty, Mongo generates one automatically.
	ID          string            `json:"id,omitempty" bson:"id,omitempty"`
	Name        string            `json:"recept_neve" bson:"recept_neve"`
	Ingridients map[string]string `json:"hozzavalok" bson:"hozzavalok"`
	Description string            `json:"elkeszites" bson:"elkeszites"`
	Comments    []Comment         `json:"comments,omitempty" bson:"comments,omitempty"`
}

func (r Recept) GetAvrageRating() float32 {
	if len(r.Comments) == 0 {
		return 0
	}
	sum := 0
	for _, komm := range r.Comments {
		sum += komm.Stars
	}
	return float32(sum) / float32(len(r.Comments))
}
func (r Recept) GetCommentCount() int {
	return len(r.Comments)
}

type Comment struct {
	ID      string `json:"id,omitempty" bson:"id,omitempty"`
	Stars   int    `json:"stars,omitempty" bson:"stars,omitempty"`
	Comment string `json:"comment,omitempty" bson:"comment,omitempty"`
}

var mongoClient *mongo.Client

func ConnectDatabase() {
	// Good practice: Use a context with a timeout for connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	fmt.Println(os.Getenv("DBURL"))
	// Connect to MongoDB
	client, err := mongo.Connect(options.Client().ApplyURI(os.Getenv("DBURL")))
	if err != nil {
		log.Fatal(err)
	}

	// Verify the connection (Ping)
	// Note: In v2, Connect might not ping automatically, so explicit ping is safer
	if err := client.Ping(ctx, nil); err != nil {
		log.Fatal("Could not ping MongoDB:", err)
	}

	fmt.Println("Connected to MongoDB!")
	mongoClient = client
}

func AddCommentToRecept(receptID string, comment Comment) error {
	coll := mongoClient.Database("receptify").Collection("recepts")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	update := bson.D{{"$push", bson.D{{"comments", comment}}}}
	res, err := coll.UpdateOne(ctx, bson.M{"id": receptID}, update)
	if err != nil {
		fmt.Println("Something wrong with appending comment: ", err)
		return err
	}
	fmt.Printf("Inserted komment: %v\n", res)
	return nil

}

// SaveRecept inserts the struct into the database
func SaveRecept(r Recept) (*mongo.InsertOneResult, error) {
	fmt.Print("heelo\n")
	uuid := uuid.New()
	r.ID = uuid.String()
	coll := mongoClient.Database("receptify").Collection("recepts")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	r.Comments = []Comment{}
	// InsertOne returns a result containing the new ID and an error if one occurred
	result, err := coll.InsertOne(ctx, r)
	if err != nil {
		return nil, fmt.Errorf("failed to insert recept: %w", err)
	}

	return result, nil
}

func GetAllRecepts() ([]Recept, error) {
	coll := mongoClient.Database("receptify").Collection("recepts")

	// 1. Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 2. Find all documents
	// bson.M{} is an empty map, meaning "no filter" (match everything)
	cursor, err := coll.Find(ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("failed to find documents: %w", err)
	}
	// 3. Ensure the cursor is closed when we are done
	defer cursor.Close(ctx)

	// 4. Decode all results into a slice of Recepts
	var recepts []Recept
	if err = cursor.All(ctx, &recepts); err != nil {
		return nil, fmt.Errorf("failed to decode documents: %w", err)
	}
	return recepts, nil
}

func GetReceptByID(recept_id string) (Recept, error) {
	coll := mongoClient.Database("receptify").Collection("recepts")

	// 1. Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var res Recept

	result := coll.FindOne(ctx, bson.M{"id": recept_id})
	if err := result.Decode(&res); err != nil {
		return res, err
	}
	return res, nil
}
