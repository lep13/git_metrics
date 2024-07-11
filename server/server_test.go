package server

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ShreerajShettyK/git_metrics/config"
	"github.com/ShreerajShettyK/git_metrics/internal/db"
	"github.com/ShreerajShettyK/git_metrics/internal/gitmetrics"
	"github.com/machinebox/graphql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MockGraphQLClient is a mock implementation of the GraphQLClient interface
type MockGraphQLClient struct {
	mock.Mock
}

func (m *MockGraphQLClient) Run(ctx context.Context, req *graphql.Request, respData interface{}) error {
	args := m.Called(ctx, req, respData)
	if fn, ok := respData.(*struct {
		User struct {
			Repositories struct {
				Nodes []gitmetrics.Repository `json:"nodes"`
			} `json:"repositories"`
		} `json:"user"`
	}); ok {
		fn.User.Repositories.Nodes = []gitmetrics.Repository{{Name: "repo1"}}
	}
	if fn, ok := respData.(*struct {
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
	}); ok {
		fn.Repository.Ref.Target.History.Nodes = []struct {
			Oid     string "json:\"oid\""
			Message string "json:\"message\""
			Author  struct {
				Name string    "json:\"name\""
				Date time.Time "json:\"date\""
			} "json:\"author\""
			Additions    int "json:\"additions\""
			Deletions    int "json:\"deletions\""
			ChangedFiles int "json:\"changedFiles\""
		}{
			{
				Oid:     "commit1",
				Message: "Initial commit",
				Author: struct {
					Name string    "json:\"name\""
					Date time.Time "json:\"date\""
				}{
					Name: "author1",
					Date: time.Now(),
				},
				Additions:    10,
				Deletions:    2,
				ChangedFiles: 1,
			},
		}
	}
	return args.Error(0)
}

// MockCollection is a mock type for the mongo.Collection used for testing.
type MockCollection struct {
	mock.Mock
}

func (m *MockCollection) UpdateOne(ctx context.Context, filter interface{}, update interface{}, opts ...*options.UpdateOptions) (*mongo.UpdateResult, error) {
	args := m.Called(ctx, filter, update, opts)
	return args.Get(0).(*mongo.UpdateResult), args.Error(1)
}

// MockRoundTripper is a mock implementation of the RoundTripper interface.
type mockRoundTripper struct{}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader([]byte(`{"files":[{"status":"added"},{"status":"removed"},{"status":"modified"}]}`))),
		Header:     make(http.Header),
	}, nil
}

var testServer *http.Server

func startTestServer() {
	go func() {
		StartServer()
	}()
	time.Sleep(1 * time.Second) // Give the server a moment to start
}

func stopTestServer() {
	if testServer != nil {
		if err := testServer.Shutdown(context.Background()); err != nil {
			log.Fatalf("Could not shut down server: %v", err)
		}
	}
}

func TestServer(t *testing.T) {
	// Mock LoadConfigFunc and InitializeMongoDBFunc
	LoadConfigFunc = func() (*config.Config, error) {
		return &config.Config{
			GitHubToken: "mock_token",
			MongoDBURI:  "mock_uri",
		}, nil
	}
	InitializeMongoDBFunc = func(uri string) error {
		return nil
	}

	// Set up mock GraphQL client
	mockGraphQLClient := &MockGraphQLClient{}
	mockGraphQLClient.On("Run", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// Set up mock HTTP client
	mockHTTPClient := &http.Client{
		Transport: &mockRoundTripper{},
	}

	// Set up mock MongoDB collection
	mockCollection := &MockCollection{}
	mockCollection.On("UpdateOne", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&mongo.UpdateResult{}, nil)

	// Mock the GetCollectionFunc to return mockCollection
	db.GetCollectionFunc = func() db.CollectionInterface {
		return mockCollection
	}

	// Start the server
	startTestServer()
	defer stopTestServer()

	t.Run("TestStartServer", func(t *testing.T) {
		req, err := http.NewRequest("GET", "http://localhost:8080/commits?user=testuser", nil)
		assert.NoError(t, err)

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := r.URL.Query().Get("user")
			if user == "" {
				http.Error(w, "Missing user parameter", http.StatusBadRequest)
				return
			}

			repositories, err := gitmetrics.FetchRepositoriesSimple(mockGraphQLClient, user, "mock_token")
			if err != nil {
				http.Error(w, fmt.Sprintf("could not fetch repositories: %v", err), http.StatusInternalServerError)
				return
			}

			for _, repo := range repositories {
				commits, err := gitmetrics.FetchCommits(mockGraphQLClient, mockHTTPClient, user, repo.Name, "mock_token")
				if err != nil {
					log.Printf("could not fetch commits for repo %s: %v", repo.Name, err)
					continue
				}

				err = gitmetrics.SaveCommitsToDB(commits)
				if err != nil {
					log.Printf("could not save commits for repo %s: %v", repo.Name, err)
					continue
				}
			}

			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "Commits fetched and stored in MongoDB successfully.")
		})

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "Commits fetched and stored in MongoDB successfully.", rr.Body.String())
	})

	// t.Run("TestStartServer_MissingUser", func(t *testing.T) {
	// 	req, err := http.NewRequest("GET", "http://localhost:8080/commits", nil)
	// 	assert.NoError(t, err)

	// 	rr := httptest.NewRecorder()
	// 	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	// 		user := r.URL.Query().Get("user")
	// 		if user == "" {
	// 			http.Error(w, "Missing user parameter", http.StatusBadRequest)
	// 			return
	// 		}
	// 	})

	// 	handler.ServeHTTP(rr, req)

	// 	assert.Equal(t, http.StatusBadRequest, rr.Code)
	// 	assert.Equal(t, "Missing user parameter\n", rr.Body.String())
	// })

	// t.Run("TestStartServer_FetchRepositoriesError", func(t *testing.T) {
	// 	// Override the mock to simulate an error
	// 	mockGraphQLClient.On("Run", mock.Anything, mock.Anything, mock.Anything).Return(errors.New("graphql error"))

	// 	req, err := http.NewRequest("GET", "http://localhost:8080/commits?user=testuser", nil)
	// 	assert.NoError(t, err)

	// 	rr := httptest.NewRecorder()
	// 	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	// 		user := r.URL.Query().Get("user")
	// 		if user == "" {
	// 			http.Error(w, "Missing user parameter", http.StatusBadRequest)
	// 			return
	// 		}

	// 		_, err := gitmetrics.FetchRepositoriesSimple(mockGraphQLClient, user, "mock_token")
	// 		if err != nil {
	// 			http.Error(w, fmt.Sprintf("could not fetch repositories: %v", err), http.StatusInternalServerError)
	// 			return
	// 		}
	// 	})

	// 	handler.ServeHTTP(rr, req)

	// 	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	// 	assert.Equal(t, "could not fetch repositories: graphql error\n", rr.Body.String())
	// })
}

