package db

import (
	"context"
	"database/sql"
	"log"
	"time"

	"github.com/go-sql-driver/mysql"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoDBClient struct {
	client   *mongo.Client
	database *mongo.Database
}

func NewMongoDBStorage(uri string, dbName string) (*MongoDBClient, error) {
	// Configura la URI de conexión a MongoDB
	clientoptions := options.Client().ApplyURI(uri)

	//Establece parametros de conexion (tiempo de espera)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Conéctate a MongoDB
	client, err := mongo.Connect(ctx, clientoptions)
	if err != nil {
		log.Fatal(err)
	}

	err = client.Ping(ctx, nil)
	if err != nil {
		return nil, err
	}

	log.Println("DB: Successfully connected")
	database := client.Database(dbName)
	return &MongoDBClient{client: client, database: database}, nil
}

func (m *MongoDBClient) GetDatabase() *mongo.Database {
	return m.database
}

func NewMySQLStorage(cfg mysql.Config) (*sql.DB, error) {
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		log.Fatal(err)
	}

	return db, nil
}
