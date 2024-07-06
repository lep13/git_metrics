package main

import (
	"fmt"
	"log"

	"git_metrics_project/config"
	"git_metrics_project/internal/db"
	"git_metrics_project/internal/gitmetrics"
)

func main() {
	// Load Config
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("could not load config: %v", err)
	}

	// Initialize MongoDB connection
	err = db.InitializeMongoDB(cfg.MongoDBURI)
	if err != nil {
		log.Fatalf("could not initialize MongoDB: %v", err)
	}

	// Example call to get commit metrics
	metrics, err := gitmetrics.GetCommitMetrics("your-repo-name", cfg.GitHubToken)
	if err != nil {
		log.Fatalf("could not get commit metrics: %v", err)
	}

	fmt.Printf("Commit Metrics: %+v\n", metrics)

	fmt.Println("Application started successfully!")
}
