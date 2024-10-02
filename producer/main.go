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
)

const (
	dbSource    = "postgres://postgres:baha123@localhost:5432/taskdb?sslmode=disable"
	maxBacklog  = 100
	produceRate = 2 // 2 tasks per second
	httpAddress = "http://localhost:8081/consume"
)

func main() {
	// Seed the random generator
	rand.Seed(time.Now().UnixNano())

	db, err := sql.Open("postgres", dbSource)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Initialize the database table
	initDB(db)

	http.Handle("/metrics", promhttp.Handler())

	go generateTasks(db)

	fmt.Println("Producer service started...")
	log.Fatal(http.ListenAndServe(":8080", nil))
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
		if backlogCount(db) < maxBacklog {
			taskType := rand.Intn(10)
			taskValue := rand.Intn(100)
			insertTask(db, taskType, taskValue)

			// Log the task generation
			log.Printf("Generated task with Type: %d, Value: %d\n", taskType, taskValue)

			// Ensure the task is inserted before the consumer tries to process it
			time.Sleep(100 * time.Millisecond)

			// Send to consumer
			_, err := http.PostForm(httpAddress, url.Values{
				"type":  {fmt.Sprint(taskType)},
				"value": {fmt.Sprint(taskValue)},
			})
			if err != nil {
				log.Println("Error sending task to consumer:", err)
			}

			// Control the rate of task generation
			time.Sleep(time.Second / time.Duration(produceRate))
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
