package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/time/rate"
	"gopkg.in/yaml.v2"
)

type Config struct {
	Prometheus struct {
		Port     int    `yaml:"port"`
		Endpoint string `yaml:"endpoint"`
	} `yaml:"prometheus"`
	TaskConsumption struct {
		MessageRate int `yaml:"message_rate"`
		BurstLimit  int `yaml:"burst_limit"`
	} `yaml:"task_consumption"`
	Logging struct {
		Level  string `yaml:"level"`
		Format string `yaml:"format"`
	} `yaml:"logging"`
	Profiling struct {
		Port int `yaml:"port"`
	} `yaml:"profiling"`
	Database struct {
		Source string `yaml:"source"`
	} `yaml:"database"`
}

var config Config

func main() {
	// Load configuration from YAML
	loadConfig()

	// Set up database connection
	db, err := sql.Open("postgres", config.Database.Source)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	limiter := rate.NewLimiter(rate.Every(time.Second/time.Duration(config.TaskConsumption.MessageRate)), config.TaskConsumption.BurstLimit)

	http.HandleFunc("/consume", func(w http.ResponseWriter, r *http.Request) {
		if !limiter.Allow() {
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		r.ParseForm()
		taskType, err := strconv.Atoi(r.FormValue("type"))
		if err != nil {
			http.Error(w, "Invalid task type", http.StatusBadRequest)
			log.Println("Error parsing task type:", err)
			return
		}
		taskValue, err := strconv.Atoi(r.FormValue("value"))
		if err != nil {
			http.Error(w, "Invalid task value", http.StatusBadRequest)
			log.Println("Error parsing task value:", err)
			return
		}

		// Find the task in the database with the received type and value
		taskID := findTaskID(db, taskType, taskValue)
		if taskID == 0 {
			http.Error(w, "Task not found", http.StatusNotFound)
			log.Printf("Task not found with type: %d and value: %d\n", taskType, taskValue)
			return
		}

		log.Printf("Processing task with ID: %d, Type: %d, Value: %d\n", taskID, taskType, taskValue)

		// Update the task state to 'processing'
		updateTaskState(db, taskID, "processing")

		// Simulate processing by sleeping for taskValue milliseconds
		time.Sleep(time.Duration(taskValue) * time.Millisecond)

		// Update the task state to 'done'
		updateTaskState(db, taskID, "done")

		log.Printf("Task with ID: %d processed successfully\n", taskID)
		fmt.Fprintf(w, "Task processed")
	})

	// Register the Prometheus handler only once
	http.Handle(config.Prometheus.Endpoint, promhttp.Handler())

	fmt.Printf("Consumer service started on port %d...\n", config.Prometheus.Port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", config.Prometheus.Port), nil))
}

// findTaskID retrieves the task ID based on the type and value with state 'received'
func findTaskID(db *sql.DB, taskType, taskValue int) int {
	var id int
	err := db.QueryRow("SELECT id FROM tasks WHERE type = $1 AND value = $2 AND state = 'received' LIMIT 1", taskType, taskValue).Scan(&id)
	if err != nil && err != sql.ErrNoRows {
		log.Println("Error finding task:", err)
		return 0
	}
	return id
}

// updateTaskState updates the state of a task in the database
func updateTaskState(db *sql.DB, id int, state string) {
	result, err := db.Exec("UPDATE tasks SET state = $1, last_update_time = NOW() WHERE id = $2", state, id)
	if err != nil {
		log.Println("Error updating task:", err)
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Println("Error checking rows affected:", err)
		return
	}

	if rowsAffected == 0 {
		log.Printf("No rows updated for task with ID: %d\n", id)
	}
}

func loadConfig() {
	file, err := os.Open("consumer_config.yaml")
	if err != nil {
		log.Fatalf("Error loading config file: %v", err)
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		log.Fatalf("Error decoding config file: %v", err)
	}
}
