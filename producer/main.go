package main

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"time"

	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/viper"
)

type Config struct {
	Prometheus struct {
		Port     int
		Endpoint string
	}
	Communication struct {
		Service2URL string
		Protocol    string
	}
	MaxBacklog int
	Logging    struct {
		Level  string
		Format string
	}
	Profiling struct {
		Port int
	}
	MessageProduction struct {
		Rate int
	}
	Database struct {
		ConnectionURL string
	}
}

var config Config

func main() {
	// Load configuration
	err := loadConfig()
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	db, err := sql.Open("postgres", config.Database.ConnectionURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Initialize the database table
	initDB(db)

	http.Handle(config.Prometheus.Endpoint, promhttp.Handler())

	go generateTasks(db)

	fmt.Printf("Producer service started on port %d...\n", config.Prometheus.Port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", config.Prometheus.Port), nil))
}

// initDB creates the tasks table if it does not exist
func initDB(db *sql.DB) {
	createTableQuery := `
	CREATE TABLE IF NOT EXISTS tasks (
		id SERIAL PRIMARY KEY,
		type INT NOT NULL CHECK (type BETWEEN 0 AND 9),
		value INT NOT NULL CHECK (value BETWEEN 0 AND 99),
		state TEXT NOT NULL CHECK (state IN ('received', 'processing', 'done')),
		creation_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		last_update_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`

	_, err := db.Exec(createTableQuery)
	if err != nil {
		log.Fatalf("Error creating tasks table: %v", err)
	}
	log.Println("Database initialized successfully")
}

func generateTasks(db *sql.DB) {
	for {
		if backlogCount(db) < config.MaxBacklog {
			taskType := rand.Intn(10)
			taskValue := rand.Intn(100)
			insertTask(db, taskType, taskValue)

			// Log the task generation
			log.Printf("Generated task with Type: %d, Value: %d\n", taskType, taskValue)

			// Send to consumer
			_, err := http.PostForm(config.Communication.Service2URL, url.Values{"type": {fmt.Sprint(taskType)}, "value": {fmt.Sprint(taskValue)}})
			if err != nil {
				log.Println("Error sending task to consumer:", err)
			}

			time.Sleep(time.Second / time.Duration(config.MessageProduction.Rate))
		}
	}
}

func backlogCount(db *sql.DB) int {
	var count int
	row := db.QueryRow("SELECT COUNT(*) FROM tasks WHERE state = 'received'")
	row.Scan(&count)
	return count
}

func insertTask(db *sql.DB, taskType, value int) {
	_, err := db.Exec("INSERT INTO tasks (type, value, state) VALUES ($1, $2, 'received')", taskType, value)
	if err != nil {
		log.Println("Error inserting task:", err)
	}
}

// loadConfig loads the configuration from a YAML file
func loadConfig() error {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("../producer/config")

	err := viper.ReadInConfig()
	if err != nil {
		return err
	}

	err = viper.Unmarshal(&config)
	if err != nil {
		return err
	}

	return nil
}
