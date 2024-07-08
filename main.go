package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/ShreerajShettyK/git_metrics/config"
	"github.com/ShreerajShettyK/git_metrics/internal/db"
	"github.com/ShreerajShettyK/git_metrics/internal/gitmetrics"
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

	http.HandleFunc("/commits", func(w http.ResponseWriter, r *http.Request) {
		user := r.URL.Query().Get("user")
		if user == "" {
			http.Error(w, "Missing user parameter", http.StatusBadRequest)
			return
		}

		repositories, err := gitmetrics.FetchAllCommits(user, cfg.GitHubToken)
		if err != nil {
			http.Error(w, fmt.Sprintf("could not get commit metrics: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(repositories)
	})

	fmt.Println("Server is running on port 8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
