package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"time"

	_ "net/http/pprof"

	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gopkg.in/yaml.v2"
)

type Config struct {
	Prometheus struct {
		Port     int    `yaml:"port"`
		Endpoint string `yaml:"endpoint"`
	} `yaml:"prometheus"`
	Communication struct {
		ConsumerURL string `yaml:"consumer_url"`
	} `yaml:"communication"`
	TaskProduction struct {
		MaxBacklog  int `yaml:"max_backlog"`
		MessageRate int `yaml:"message_rate"`
	} `yaml:"task_production"`
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

var (
	config               Config
	version              = "1.0.1" // Define your version here
	tasksProducedCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "tasks_produced_total",
		Help: "Total number of tasks produced",
	}, []string{"type"})
)

func main() {
	// Check for version flag
	versionFlag := flag.Bool("version", false, "Display the version")
	flag.Parse()

	if *versionFlag {
		fmt.Println("Producer Service Version:", version)
		os.Exit(0)
	}

	// Load configuration from YAML
	loadConfig()

	// Register Prometheus metrics
	prometheus.MustRegister(tasksProducedCounter)

	// Seed the random generator
	rand.Seed(time.Now().UnixNano())

	// Set up database connection
	db, err := sql.Open("postgres", config.Database.Source)
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
		if backlogCount(db) < config.TaskProduction.MaxBacklog {
			taskType := rand.Intn(10)
			taskValue := rand.Intn(100)
			insertTask(db, taskType, taskValue)

			// Log the task generation
			log.Printf("Generated task with Type: %d, Value: %d\n", taskType, taskValue)

			// Increment the tasks produced counter
			tasksProducedCounter.WithLabelValues(fmt.Sprint(taskType)).Inc()

			time.Sleep(100 * time.Millisecond)

			// Send to consumer
			_, err := http.PostForm(config.Communication.ConsumerURL, url.Values{
				"type":  {fmt.Sprint(taskType)},
				"value": {fmt.Sprint(taskValue)},
			})
			if err != nil {
				log.Println("Error sending task to consumer:", err)
			}

			// Control the rate of task generation
			time.Sleep(time.Second / time.Duration(config.TaskProduction.MessageRate))
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

func loadConfig() {
	file, err := os.Open("producer_config.yaml")
	if err != nil {
		log.Fatalf("Error loading config file: %v", err)
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		log.Fatalf("Error decoding config file: %v", err)
	}
}
