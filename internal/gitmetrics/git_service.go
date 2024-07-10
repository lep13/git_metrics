package gitmetrics

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/ShreerajShettyK/git_metrics/internal/db"
	"github.com/machinebox/graphql"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type GraphQLClient interface {
	Run(ctx context.Context, req *graphql.Request, respData interface{}) error
}

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

func FetchRepositoriesSimple(client GraphQLClient, user, token string) ([]Repository, error) {
	req := graphql.NewRequest(`
		query($user: String!) {
			user(login: $user) {
				repositories(first: 100) {
					nodes {
						name
					}
				}
			}
		}
	`)

	req.Var("user", user)
	req.Header.Set("Authorization", "Bearer "+token)

	var respData struct {
		User struct {
			Repositories struct {
				Nodes []Repository `json:"nodes"`
			} `json:"repositories"`
		} `json:"user"`
	}

	if err := client.Run(context.Background(), req, &respData); err != nil {
		return nil, err
	}

	return respData.User.Repositories.Nodes, nil
}

func FetchCommits(client GraphQLClient, httpClient HTTPClient, user, repo, token string) ([]Commit, error) {
	req := graphql.NewRequest(`
		query($user: String!, $repo: String!) {
			repository(owner: $user, name: $repo) {
				ref(qualifiedName: "main") {
					target {
						... on Commit {
							history(first: 100) {
								nodes {
									oid
									message
									author {
										name
										date
									}
									additions
									deletions
									changedFiles
								}
							}
						}
					}
				}
			}
		}
	`)

	req.Var("user", user)
	req.Var("repo", repo)
	req.Header.Set("Authorization", "Bearer "+token)

	var respData struct {
		Repository struct {
			Ref struct {
				Target struct {
					History struct {
						Nodes []struct {
							Oid     string `json:"oid"`
							Message string `json:"message"`
							Author  struct {
								Name string    `json:"name"`
								Date time.Time `json:"date"`
							} `json:"author"`
							Additions    int `json:"additions"`
							Deletions    int `json:"deletions"`
							ChangedFiles int `json:"changedFiles"`
						} `json:"nodes"`
					} `json:"history"`
				} `json:"target"`
			} `json:"ref"`
		} `json:"repository"`
	}

	if err := client.Run(context.Background(), req, &respData); err != nil {
		return nil, err
	}

	var commits []Commit

	for _, node := range respData.Repository.Ref.Target.History.Nodes {
		commit := Commit{
			CommitMessage: node.Message,
			LinesDeleted:  node.Deletions,
			CommitID:      node.Oid,
			CommittedBy:   node.Author.Name,
			LinesAdded:    node.Additions,
			RepoName:      repo,
			CommitDate:    node.Author.Date,
		}

		filesAdded, filesDeleted, filesUpdated, err := FetchCommitFileChanges(httpClient, user, repo, node.Oid, token)
		if err != nil {
			log.Printf("failed to fetch file changes for commit %s: %v", node.Oid, err)
		} else {
			commit.FilesAdded = filesAdded
			commit.FilesDeleted = filesDeleted
			commit.FilesUpdated = filesUpdated
		}

		commits = append(commits, commit)
	}

	return commits, nil
}

func FetchCommitFileChanges(client HTTPClient, user, repo, commitID, token string) (int, int, int, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/commits/%s", user, repo, commitID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, 0, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return 0, 0, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, 0, 0, fmt.Errorf("failed to fetch commit details: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, 0, 0, err
	}

	var commitData struct {
		Files []struct {
			Status string `json:"status"`
		} `json:"files"`
	}

	if err := json.Unmarshal(body, &commitData); err != nil {
		return 0, 0, 0, err
	}

	filesAdded, filesDeleted, filesUpdated := 0, 0, 0
	for _, file := range commitData.Files {
		switch file.Status {
		case "added":
			filesAdded++
		case "removed":
			filesDeleted++
		case "modified":
			filesUpdated++
		}
	}

	return filesAdded, filesDeleted, filesUpdated, nil
}

func SaveCommitsToDB(commits []Commit) error {
	collection := db.GetCollection()

	for _, commit := range commits {
		filter := bson.M{"commit_id": commit.CommitID}
		update := bson.M{"$setOnInsert": commit}
		opts := options.Update().SetUpsert(true)
		_, err := collection.UpdateOne(context.Background(), filter, update, opts)
		if err != nil {
			return fmt.Errorf("failed to update commit: %w", err)
		}
	}

	return nil
}
