package gitmetrics

import (
	"context"
	"fmt"
	"time"

	"github.com/ShreerajShettyK/git_metrics/internal/db"
	"github.com/machinebox/graphql"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Commit struct {
	CommitMessage string    `bson:"commit_message"`
	LinesDeleted  int       `bson:"lines_deleted"`
	CommitID      string    `bson:"commit_id"`
	CommittedBy   string    `bson:"commited_by"`
	LinesAdded    int       `bson:"lines_added"`
	RepoName      string    `bson:"reponame"`
	CommitDate    time.Time `bson:"commit_date"`
	FilesAdded    int       `bson:"files_added"`
	FilesDeleted  int       `bson:"files_deleted"`
	FilesUpdated  int       `bson:"files_updated"`
}

type Repository struct {
	Name string `json:"name"`
}

func FetchRepositoriesSimple(user, token string) ([]Repository, error) {
	client := graphql.NewClient("https://api.github.com/graphql")
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

func FetchCommits(user, repo, token string) ([]Commit, error) {
	client := graphql.NewClient("https://api.github.com/graphql")
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
									associatedPullRequests(first: 1) {
										nodes {
											files(first: 100) {
												nodes {
													additions
													deletions
													path
													changeType
												}
											}
										}
									}
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
							Additions              int `json:"additions"`
							Deletions              int `json:"deletions"`
							ChangedFiles           int `json:"changedFiles"`
							AssociatedPullRequests struct {
								Nodes []struct {
									Files struct {
										Nodes []struct {
											Additions  int    `json:"additions"`
											Deletions  int    `json:"deletions"`
											Path       string `json:"path"`
											ChangeType string `json:"changeType"`
										} `json:"nodes"`
									} `json:"files"`
								} `json:"nodes"`
							} `json:"associatedPullRequests"`
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

		for _, pr := range node.AssociatedPullRequests.Nodes {
			for _, file := range pr.Files.Nodes {
				switch file.ChangeType {
				case "ADDED":
					commit.FilesAdded++
				case "MODIFIED":
					commit.FilesUpdated++
				case "REMOVED":
					commit.FilesDeleted++
				}
			}
		}

		commits = append(commits, commit)
	}

	return commits, nil
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
