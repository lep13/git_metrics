package gitmetrics

import (
	"context"
	"fmt"
	"log"

	"github.com/machinebox/graphql"
)

type Repository struct {
	Name    string   `json:"name"`
	Commits []Commit `json:"commits"`
}

type Commit struct {
	SHA     string `json:"sha"`
	Message string `json:"message"`
	Author  struct {
		Name string `json:"name"`
		Date string `json:"date"`
	} `json:"author"`
	Additions    int `json:"additions"`
	Deletions    int `json:"deletions"`
	ChangedFiles int `json:"changedFiles"`
}

// fetchRepositoriesSimple fetches a list of repositories for a given user
func fetchRepositoriesSimple(user string, gitHubToken string) ([]Repository, error) {
	client := graphql.NewClient("https://api.github.com/graphql")

	req := graphql.NewRequest(`
		query($login: String!) {
			user(login: $login) {
				repositories(first: 10) {
					nodes {
						name
					}
				}
			}
		}
	`)

	req.Var("login", user)
	req.Header.Set("Authorization", "Bearer "+gitHubToken)

	var respData struct {
		User struct {
			Repositories struct {
				Nodes []Repository `json:"nodes"`
			} `json:"repositories"`
		} `json:"user"`
	}

	ctx := context.Background()
	if err := client.Run(ctx, req, &respData); err != nil {
		return nil, fmt.Errorf("failed to fetch repositories: %w", err)
	}

	return respData.User.Repositories.Nodes, nil
}

func fetchCommitsWithPagination(user, repoName, gitHubToken string) ([]Commit, error) {
	client := graphql.NewClient("https://api.github.com/graphql")

	var commits []Commit
	var cursor *string

	for {
		req := graphql.NewRequest(`
			query($login: String!, $repoName: String!, $cursor: String) {
				repository(owner: $login, name: $repoName) {
					defaultBranchRef {
						target {
							... on Commit {
								history(first: 20, after: $cursor) {
									edges {
										node {
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
		`)

		req.Var("login", user)
		req.Var("repoName", repoName)
		req.Var("cursor", cursor)
		req.Header.Set("Authorization", "Bearer "+gitHubToken)

		var respData struct {
			Repository struct {
				DefaultBranchRef struct {
					Target struct {
						History struct {
							Edges []struct {
								Node struct {
									OID     string `json:"oid"`
									Message string `json:"message"`
									Author  struct {
										Name string `json:"name"`
										Date string `json:"date"`
									} `json:"author"`
									Additions    int `json:"additions"`
									Deletions    int `json:"deletions"`
									ChangedFiles int `json:"changedFiles"`
								} `json:"node"`
							} `json:"edges"`
							PageInfo struct {
								HasNextPage bool   `json:"hasNextPage"`
								EndCursor   string `json:"endCursor"`
							} `json:"pageInfo"`
						} `json:"history"`
					} `json:"target"`
				} `json:"defaultBranchRef"`
			} `json:"repository"`
		}

		ctx := context.Background()
		if err := client.Run(ctx, req, &respData); err != nil {
			return nil, fmt.Errorf("failed to fetch commits: %w", err)
		}

		for _, commitEdge := range respData.Repository.DefaultBranchRef.Target.History.Edges {
			commitNode := commitEdge.Node
			commit := Commit{
				SHA:          commitNode.OID,
				Message:      commitNode.Message,
				Author:       commitNode.Author,
				Additions:    commitNode.Additions,
				Deletions:    commitNode.Deletions,
				ChangedFiles: commitNode.ChangedFiles,
			}
			commits = append(commits, commit)
		}

		if !respData.Repository.DefaultBranchRef.Target.History.PageInfo.HasNextPage {
			break
		}

		cursor = &respData.Repository.DefaultBranchRef.Target.History.PageInfo.EndCursor
	}

	return commits, nil
}

func FetchAllCommits(user string, gitHubToken string) ([]Repository, error) {
	repositories, err := fetchRepositoriesSimple(user, gitHubToken)
	if err != nil {
		return nil, err
	}

	for i, repo := range repositories {
		commits, err := fetchCommitsWithPagination(user, repo.Name, gitHubToken)
		if err != nil {
			log.Printf("failed to get commits for repo %s: %v", repo.Name, err)
			continue
		}
		repositories[i].Commits = commits
	}

	return repositories, nil
}
