package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"regexp"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

var masterKey string
var port = ":8080"

func loadEnvVars() {
	if err := godotenv.Load(".env"); err != nil {
		logger.Fatal("Error loading .env files")
	}

	var ok bool
	if masterKey, ok = os.LookupEnv("EULM_FILES_MASTER_KEY"); !ok {
		logger.Fatal("Environment variable EULM_FILES_MASTER_KEY not found")
	}

	if portVar := os.Getenv("EULM_FILES_PORT"); regexp.MustCompile(`^:\d{4}$`).MatchString(portVar) {
		port = portVar
	}

	logger.Info("Loaded variables from .env files")
}

func initDB() {
	var err error

	if err = os.MkdirAll("db", os.ModePerm); err != nil {
		logger.Fatal("Error creating database directory:", err.Error())
	}

	// SQLite driver creates the database file as long as its parent directory exists
	if db, err = sql.Open("sqlite3", "db/main.db"); err != nil {
		logger.Fatal("Error opening database:", err.Error())
	}
	if err = db.Ping(); err != nil {
		logger.Fatal("Error connecting to database:", err.Error())
	}

	if _, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS files (
            id TEXT NOT NULL,
            file_name TEXT NOT NULL,
			uploaded_at TEXT NOT NULL,
			creator TEXT NOT NULL
        );
        CREATE TABLE IF NOT EXISTS users (
            api_key TEXT NOT NULL,
            username TEXT UNIQUE NOT NULL,
            permissions INTEGER NOT NULL
        );
    `); err != nil {
		logger.Fatal("Error creating tables:", err.Error())
	}

	if _, err = db.Exec("DELETE FROM users WHERE username = ?", "Master"); err != nil {
		logger.Fatal("Error deleting existing Master user(s):", err.Error())
	}
	if _, err = db.Exec("INSERT INTO users (api_key, username, permissions) VALUES (?, ?, ?)", masterKey, "Master", 3); err != nil {
		logger.Fatal("Error inserting new Master user:", err.Error())
	}

	logger.Info("Database initialised successfully")
}

func closeDB() {
	if err := db.Close(); err != nil {
		logger.Fatal("Error closing database connection:", err.Error())
	}
}

func main() {
	loadEnvVars()

	initDB()
	defer closeDB()

	r := mux.NewRouter()

	handleApi(r)

	r.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		respondJSON(w, http.StatusNotFound, map[string]any{"message": "Route not found"})
	})

	logger.Info(fmt.Sprintf("Server starting on port %s", port))
	if err := http.ListenAndServe(port, r); err != nil {
		logger.Fatal("Error starting server:", err.Error())
	}
}
