package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/time/rate"
	"gopkg.in/yaml.v2"

	// Importing pprof for profiling
	_ "net/http/pprof"
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

var (
	config                Config
	version               = "1.0.1" // Define your version here
	tasksProcessedCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "tasks_processed_total",
		Help: "Total number of tasks processed",
	}, []string{"type"})
	tasksDoneCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "tasks_done_total",
		Help: "Total number of tasks done",
	}, []string{"type"})
	totalValueCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "total_value_per_task_type",
		Help: "Total value of tasks processed per type",
	}, []string{"type"})
	tasksInProcessingGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "tasks_in_processing",
		Help: "Current number of tasks being processed",
	}, []string{"type"})
	taskValueSum = make(map[int]float64) // This map tracks total values per task type
	mu           sync.Mutex              // To synchronize map updates
)

func main() {
	// Check for version flag
	versionFlag := flag.Bool("version", false, "Display the version")
	flag.Parse()

	if *versionFlag {
		fmt.Println("Consumer Service Version:", version)
		os.Exit(0)
	}

	// Load configuration from YAML
	loadConfig()

	// Register Prometheus metrics
	prometheus.MustRegister(tasksProcessedCounter)
	prometheus.MustRegister(tasksDoneCounter)
	prometheus.MustRegister(totalValueCounter)
	prometheus.MustRegister(tasksInProcessingGauge)

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

		// Increment tasks in processing gauge
		tasksInProcessingGauge.WithLabelValues(fmt.Sprint(taskType)).Inc()

		// Simulate processing by sleeping for taskValue milliseconds
		time.Sleep(time.Duration(taskValue) * time.Millisecond)

		// Decrement tasks in processing gauge
		tasksInProcessingGauge.WithLabelValues(fmt.Sprint(taskType)).Dec()

		// Update the task state to 'done'
		updateTaskState(db, taskID, "done")

		log.Printf("Task with ID: %d processed successfully\n", taskID)

		// Increment the processed tasks counter and total value
		tasksProcessedCounter.WithLabelValues(fmt.Sprint(taskType)).Inc()
		tasksDoneCounter.WithLabelValues(fmt.Sprint(taskType)).Inc()
		totalValueCounter.WithLabelValues(fmt.Sprint(taskType)).Add(float64(taskValue))

		// Update the total value sum for this task type
		mu.Lock()
		taskValueSum[taskType] += float64(taskValue)
		currentTotalValue := taskValueSum[taskType]
		mu.Unlock()

		// Log final sum for task type
		log.Printf("Total value for tasks of type %d is now: %.2f", taskType, currentTotalValue)

		fmt.Fprintln(w, "Task processed successfully")
	})
	// Start pprof server in a goroutine
	go func() {
		log.Println(http.ListenAndServe(fmt.Sprintf(":%d", config.Profiling.Port), nil))
	}()

	http.Handle(config.Prometheus.Endpoint, promhttp.Handler())

	fmt.Printf("Consumer service started on port %d...\n", config.Prometheus.Port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", config.Prometheus.Port), nil))
}

func findTaskID(db *sql.DB, taskType, value int) int {
	var taskID int
	row := db.QueryRow("SELECT id FROM tasks WHERE type = $1 AND value = $2 AND state = 'received'", taskType, value)
	err := row.Scan(&taskID)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0 // No task found
		}
		log.Printf("Error querying task ID: %v\n", err)
	}
	return taskID
}

func updateTaskState(db *sql.DB, taskID int, state string) {
	_, err := db.Exec("UPDATE tasks SET state = $1, last_update_time = CURRENT_TIMESTAMP WHERE id = $2", state, taskID)
	if err != nil {
		log.Printf("Error updating task state: %v\n", err)
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
