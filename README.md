
# Git Metrics Service

## Overview

The Git Metrics Service is a Go-based application that fetches commit information from a specified GitHub user's repositories using the GitHub GraphQL API and stores this information in a MongoDB database. This service is designed to efficiently fetch commit data by leveraging GraphQL queries to reduce the number of API calls required.

## Features

- Fetches commit data from GitHub repositories using GraphQL API.
- Retrieves detailed information about each commit, including files changed.
- Stores commit information in a MongoDB database.
- Uses AWS Secrets Manager to securely manage GitHub personal access token and MongoDB connection URI.

## Prerequisites

- Go 1.16 or later
- MongoDB instance
- AWS account with Secrets Manager configured

## Installation

1. Clone the repository:

   ```sh
   git clone https://github.com/ShreerajShettyK/git_metrics.git
   cd git_metrics
   ```

2. Install the dependencies:

   ```sh
   go mod tidy
   ```

3. Set up your AWS Secrets Manager with the following secrets:
   - `github_token`: Your GitHub personal access token.
   - `mongodb_uri`: Your MongoDB connection string.

## Configuration

The application loads its configuration from AWS Secrets Manager. Ensure you have the necessary AWS credentials configured on your local machine or deployment environment.

## Running the Application

Start the server with the following command:

```sh
go run main.go
```

The server will start and listen on port `8080`.

## Endpoints

### GET /commits

Fetches commits from the specified GitHub user's repositories and stores them in MongoDB.

#### Query Parameters

- `user`: The GitHub username whose repositories' commits need to be fetched.

#### Example Request

```sh
curl "http://localhost:8080/commits?user=ShreerajShettyK"
```

## Project Structure

- `main.go`: Entry point of the application.
- `config/`: Contains configuration loading logic.
- `internal/db/`: Handles MongoDB connection and operations.
- `internal/gitmetrics/`: Contains logic for fetching commit data from GitHub and saving it to MongoDB.
- `server/`: Contains server setup and HTTP handler logic.

## GraphQL Queries

### FetchRepositories

Fetches a list of repositories for the specified user.

```graphql
query($user: String!) {
    user(login: $user) {
        repositories(first: 100) {
            nodes {
                name
            }
        }
    }
}
```

### FetchCommits

Fetches the commit history for a specified repository.

```graphql
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
```

## Commit Structure

Each commit is represented by the following structure:

```go
type Commit struct {
    CommitMessage string    `bson:"commit_message"`
    LinesDeleted  int       `bson:"lines_deleted"`
    CommitID      string    `bson:"commit_id"`
    CommittedBy   string    `bson:"committed_by"`
    LinesAdded    int       `bson:"lines_added"`
    RepoName      string    `bson:"repo_name"`
    CommitDate    time.Time `bson:"commit_date"`
    FilesAdded    int       `bson:"files_added"`
    FilesDeleted  int       `bson:"files_deleted"`
    FilesUpdated  int       `bson:"files_updated"`
}
```

## Error Handling

The application includes comprehensive error handling to ensure any issues encountered during data fetching or database operations are logged and reported appropriately.

## Testing

The application includes unit tests to verify functionality. Run the tests using:

```sh
go test ./...
```

## License

This project is licensed under the MIT License.
