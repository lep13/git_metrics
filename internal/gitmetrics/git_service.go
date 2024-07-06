package gitmetrics

import (
	"context"
	"fmt"
	"github.com/machinebox/graphql"
)

func GetCommitMetrics(repoName string, gitHubToken string) (map[string]interface{}, error) {
	client := graphql.NewClient("https://api.github.com/graphql")
	req := graphql.NewRequest(`
	query ($name: String!) {
	  repository(owner:"your-github-username", name: $name) {
	    name
	    ref(qualifiedName: "main") {
	      target {
	        ... on Commit {
	          history(first: 100) {
	            edges {
	              node {
	                committedDate
	                additions
	                deletions
	                changedFiles
	                author {
	                  name
	                }
	              }
	            }
	          }
	        }
	      }
	    }
	  }
	}`)

	req.Var("name", repoName)
	req.Header.Set("Authorization", "Bearer "+gitHubToken)

	var respData map[string]interface{}
	ctx := context.Background()

	if err := client.Run(ctx, req, &respData); err != nil {
		return nil, fmt.Errorf("failed to get commit metrics: %w", err)
	}

	return respData, nil
}
