package gitmetrics

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

// Repository represents a GitHub repository
type Repository struct {
	Name    string   `json:"name"`
	Commits []Commit `json:"commits"`
}

// Commit represents a GitHub commit
type Commit struct {
	SHA    string `json:"sha"`
	Commit struct {
		Author struct {
			Name string `json:"name"`
			Date string `json:"date"`
		} `json:"author"`
		Message string `json:"message"`
	} `json:"commit"`
	Stats struct {
		Additions int `json:"additions"`
		Deletions int `json:"deletions"`
		Total     int `json:"total"`
	} `json:"stats,omitempty"`
	Files []struct {
		Additions int    `json:"additions"`
		Deletions int    `json:"deletions"`
		Changes   int    `json:"changes"`
		Filename  string `json:"filename"`
		Status    string `json:"status"`
	} `json:"files,omitempty"`
}

// CommitDetails represents detailed information about a GitHub commit
type CommitDetails struct {
	SHA   string `json:"sha"`
	Stats struct {
		Additions int `json:"additions"`
		Deletions int `json:"deletions"`
		Total     int `json:"total"`
	} `json:"stats"`
	Files []struct {
		Additions int    `json:"additions"`
		Deletions int    `json:"deletions"`
		Changes   int    `json:"changes"`
		Filename  string `json:"filename"`
		Status    string `json:"status"`
	} `json:"files"`
}

func fetchRepositories(user string, gitHubToken string) ([]Repository, error) {
	client := &http.Client{}
	url := fmt.Sprintf("https://api.github.com/users/%s/repos", user)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+gitHubToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get repositories: %s", resp.Status)
	}

	var repositories []Repository
	if err := json.NewDecoder(resp.Body).Decode(&repositories); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return repositories, nil
}

func fetchCommits(repoName string, gitHubToken string) ([]Commit, error) {
	client := &http.Client{}
	url := fmt.Sprintf("https://api.github.com/repos/ShreerajShettyK/%s/commits", repoName)

	var allCommits []Commit
	for {
		log.Printf("Fetching commits from URL: %s", url)

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Authorization", "Bearer "+gitHubToken)
		req.Header.Set("Accept", "application/vnd.github.v3+json")

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to make request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("failed to get commits: %s", resp.Status)
		}

		var commits []Commit
		if err := json.NewDecoder(resp.Body).Decode(&commits); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		allCommits = append(allCommits, commits...)

		// Check if there's another page
		url = ""
		for _, link := range resp.Header["Link"] {
			if len(link) > 0 {
				var nextLink string
				fmt.Sscanf(link, `<%s>; rel="next"`, &nextLink)
				if nextLink != "" {
					url = nextLink
					break
				}
			}
		}
		if url == "" {
			break
		}
	}

	return allCommits, nil
}

func fetchCommitDetails(repoName string, sha string, gitHubToken string) (*CommitDetails, error) {
	client := &http.Client{}
	url := fmt.Sprintf("https://api.github.com/repos/ShreerajShettyK/%s/commits/%s", repoName, sha)

	log.Printf("Fetching commit details from URL: %s", url)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+gitHubToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get commit details: %s", resp.Status)
	}

	var commitDetails CommitDetails
	if err := json.NewDecoder(resp.Body).Decode(&commitDetails); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &commitDetails, nil
}

func GetAllCommitMetrics(user string, gitHubToken string) ([]Repository, error) {
	repositories, err := fetchRepositories(user, gitHubToken)
	if err != nil {
		return nil, err
	}

	for i, repo := range repositories {
		commits, err := fetchCommits(repo.Name, gitHubToken)
		if err != nil {
			log.Printf("failed to get commits for repo %s: %v", repo.Name, err)
			continue
		}

		for _, commit := range commits {
			details, err := fetchCommitDetails(repo.Name, commit.SHA, gitHubToken)
			if err != nil {
				log.Printf("failed to get commit details for repo %s, commit %s: %v", repo.Name, commit.SHA, err)
				continue
			}

			commit.Stats = details.Stats
			commit.Files = details.Files

			repositories[i].Commits = append(repositories[i].Commits, commit)
		}
	}

	return repositories, nil
}
