package server

import (
	"fmt"
	"log"
	"net/http"

	"github.com/ShreerajShettyK/git_metrics/config"
	"github.com/ShreerajShettyK/git_metrics/internal/db"
	"github.com/ShreerajShettyK/git_metrics/internal/gitmetrics"
	"github.com/machinebox/graphql"
)

// Function variables to allow swapping with mocks in tests
var LoadConfigFunc = config.LoadConfig
var InitializeMongoDBFunc = db.InitializeMongoDB

func StartServer() {
	// Load Config
	cfg, err := LoadConfigFunc()
	if err != nil {
		log.Fatalf("could not load config: %v", err)
	}

	// Initialize MongoDB connection
	err = InitializeMongoDBFunc(cfg.MongoDBURI)
	if err != nil {
		log.Fatalf("could not initialize MongoDB: %v", err)
	}

	graphqlClient := graphql.NewClient("https://api.github.com/graphql")
	httpClient := &http.Client{}

	http.HandleFunc("/commits", func(w http.ResponseWriter, r *http.Request) {
		user := r.URL.Query().Get("user")
		if user == "" {
			http.Error(w, "Missing user parameter", http.StatusBadRequest)
			return
		}

		repositories, err := gitmetrics.FetchRepositoriesSimple(graphqlClient, user, cfg.GitHubToken)
		if err != nil {
			http.Error(w, fmt.Sprintf("could not fetch repositories: %v", err), http.StatusInternalServerError)
			return
		}

		for _, repo := range repositories {
			commits, err := gitmetrics.FetchCommits(graphqlClient, httpClient, user, repo.Name, cfg.GitHubToken)
			if err != nil {
				log.Printf("could not fetch commits for repo %s: %v", repo.Name, err)
				continue // Skip this repository and continue with the next one
			}

			err = gitmetrics.SaveCommitsToDB(commits)
			if err != nil {
				log.Printf("could not save commits for repo %s: %v", repo.Name, err)
				continue // Skip saving this repository's commits and continue with the next one
			}
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Commits fetched and stored in MongoDB successfully.")
	})

	fmt.Println("Server is running on port 8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
