package main

import (
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

		repositories, err := gitmetrics.FetchRepositoriesSimple(user, cfg.GitHubToken)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to fetch repositories: %v", err), http.StatusInternalServerError)
			return
		}

		for _, repo := range repositories {
			commits, err := gitmetrics.FetchCommits(user, repo.Name, cfg.GitHubToken)
			if err != nil {
				log.Printf("failed to get commits for repo %s: %v", repo.Name, err)
				continue
			}

			if err := gitmetrics.SaveCommitsToDB(commits); err != nil {
				log.Printf("failed to save commits for repo %s: %v", repo.Name, err)
			}
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Commits fetched and stored in MongoDB successfully.")
	})

	fmt.Println("Server is running on port 8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
