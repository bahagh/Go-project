package main

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gopkg.in/yaml.v2"
)

func TestLoadConfig(t *testing.T) {
	file, err := os.Open("producer_config.yaml")
	if err != nil {
		t.Fatalf("Error loading config file: %v", err)
	}
	defer file.Close()

	var config Config
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		t.Fatalf("Error decoding config file: %v", err)
	}

	if config.Prometheus.Port != 8080 {
		t.Errorf("Expected Prometheus port 8080, got %d", config.Prometheus.Port)
	}
}

func TestGenerateTasks(t *testing.T) {
	// Mock database connection
	db, err := sql.Open("postgres", "postgres://postgres:baha123@localhost:5432/taskdb?sslmode=disable")
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Call the function directly or set up an HTTP handler to test it
	// Here we would test inserting and checking tasks
}

func TestMetricsEndpoint(t *testing.T) {
	req, err := http.NewRequest("GET", "/metrics", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(promhttp.Handler())
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Expected status OK, got %v", status)
	}
}
