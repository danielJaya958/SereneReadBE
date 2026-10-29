package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	ServerPort string
	DBUrl      string
}

func LoadConfig() (*Config, error) {
	// Load file .env jika ada
	// Jika running di production (Docker/Server), mungkin tidak pakai .env file tapi env vars langsung
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: .env file not found, using system environment variables")
	}

	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbUser := os.Getenv("DB_USER")
	dbPass := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")
	
	// Construct connection string untuk PostgreSQL
	// Format: postgres://user:password@host:port/dbname
	dbUrl := "postgres://" + dbUser + ":" + dbPass + "@" + dbHost + ":" + dbPort + "/" + dbName

	return &Config{
		ServerPort: os.Getenv("SERVER_PORT"),
		DBUrl:      dbUrl,
	}, nil
}