package gitmetrics

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/lep13/git_metrics/config"
	"github.com/lep13/git_metrics/internal/db"
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

type CustomGraphQLRequest struct {
	*graphql.Request
	QueryType string
}

type GitMetrics interface {
	FetchRepositoriesSimple(client *graphql.Client, user string, token string) ([]Repository, error)
	FetchCommits(client *graphql.Client, httpClient *http.Client, user string, repo string, token string) ([]Commit, error)
	SaveCommitsToDB(commits []Commit) error
}

type GitMetricsImpl struct{}

func (g *GitMetricsImpl) FetchRepositoriesSimple(client *graphql.Client, user string, token string) ([]Repository, error) {
	return FetchRepositoriesSimple(client, user, token)
}

func (g *GitMetricsImpl) FetchCommits(client *graphql.Client, httpClient *http.Client, user string, repo string, token string) ([]Commit, error) {
	return FetchCommits(client, httpClient, user, repo, token)
}

func (g *GitMetricsImpl) SaveCommitsToDB(commits []Commit) error {
	return SaveCommitsToDB(commits)
}

// const maxReposPerPage = 100
// const maxCommitsPerPage = 100

func FetchRepositoriesSimple(client GraphQLClient, user, token string) ([]Repository, error) {
	var allRepositories []Repository
	var cursor *string

	for {
		req := graphql.NewRequest(`
			query($user: String!, $cursor: String) {
				user(login: $user) {
					repositories(first: 100, after: $cursor) {
						nodes {
							name
						}
						pageInfo {
							hasNextPage
							endCursor
						}
					}
				}
			}
		`)

		req.Var("user", user)
		req.Var("cursor", cursor)
		req.Header.Set("Authorization", "Bearer "+token)

		var respData struct {
			User struct {
				Repositories struct {
					Nodes    []Repository `json:"nodes"`
					PageInfo struct {
						HasNextPage bool   `json:"hasNextPage"`
						EndCursor   string `json:"endCursor"`
					} `json:"pageInfo"`
				} `json:"repositories"`
			} `json:"user"`
		}

		if err := client.Run(context.Background(), req, &respData); err != nil {
			return nil, err
		}

		allRepositories = append(allRepositories, respData.User.Repositories.Nodes...)

		if !respData.User.Repositories.PageInfo.HasNextPage {
			break
		}
		cursor = &respData.User.Repositories.PageInfo.EndCursor
	}

	return allRepositories, nil
}

func FetchCommits(client GraphQLClient, httpClient HTTPClient, user, repo, token string) ([]Commit, error) {
	defaultBranchReq := &CustomGraphQLRequest{
		Request: graphql.NewRequest(`
			query($user: String!, $repo: String!) {
				repository(owner: $user, name: $repo) {
					defaultBranchRef {
						name
					}
				}
			}
		`),
		QueryType: "defaultBranch",
	}
	defaultBranchReq.Var("user", user)
	defaultBranchReq.Var("repo", repo)
	defaultBranchReq.Header.Set("Authorization", "Bearer "+token)

	var defaultBranchResp struct {
		Repository struct {
			DefaultBranchRef struct {
				Name string `json:"name"`
			} `json:"defaultBranchRef"`
		} `json:"repository"`
	}

	if err := client.Run(context.Background(), defaultBranchReq.Request, &defaultBranchResp); err != nil {
		return nil, err
	}
	defaultBranch := defaultBranchResp.Repository.DefaultBranchRef.Name

	var allCommits []Commit
	var cursor *string

	for {
		req := &CustomGraphQLRequest{
			Request: graphql.NewRequest(`
				query($user: String!, $repo: String!, $branch: String!, $cursor: String) {
					repository(owner: $user, name: $repo) {
						ref(qualifiedName: $branch) {
							target {
								... on Commit {
									history(first: 100, after: $cursor) {
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
										pageInfo {
											hasNextPage
											endCursor
										}
									}
								}
							}
						}
					}
				}
			`),
			QueryType: "commits",
		}

		req.Var("user", user)
		req.Var("repo", repo)
		req.Var("branch", defaultBranch)
		req.Var("cursor", cursor)
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
							PageInfo struct {
								HasNextPage bool   `json:"hasNextPage"`
								EndCursor   string `json:"endCursor"`
							} `json:"pageInfo"`
						} `json:"history"`
					} `json:"target"`
				} `json:"ref"`
			} `json:"repository"`
		}

		if err := client.Run(context.Background(), req.Request, &respData); err != nil {
			return nil, err
		}

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

			allCommits = append(allCommits, commit)
		}

		if !respData.Repository.Ref.Target.History.PageInfo.HasNextPage {
			break
		}
		cursor = &respData.Repository.Ref.Target.History.PageInfo.EndCursor
	}

	return allCommits, nil
}

// func getDefaultBranch(client GraphQLClient, user, repo, token string) (string, error) {
// 	req := graphql.NewRequest(`
// 		query($user: String!, $repo: String!) {
// 			repository(owner: $user, name: $repo) {
// 				defaultBranchRef {
// 					name
// 				}
// 			}
// 		}
// 	`)

// 	req.Var("user", user)
// 	req.Var("repo", repo)
// 	req.Header.Set("Authorization", "Bearer "+token)

// 	var respData struct {
// 		Repository struct {
// 			DefaultBranchRef struct {
// 				Name string `json:"name"`
// 			} `json:"defaultBranchRef"`
// 		} `json:"repository"`
// 	}

// 	if err := client.Run(context.Background(), req, &respData); err != nil {
// 		return "", err
// 	}

// 	return respData.Repository.DefaultBranchRef.Name, nil
// }

func FetchCommitFileChanges(client HTTPClient, user, repo, commitID, token string) (int, int, int, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to load config: %w", err)
	}

	url := fmt.Sprintf(cfg.FilesAPI, user, repo, commitID)
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
