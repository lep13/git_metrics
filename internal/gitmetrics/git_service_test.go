package gitmetrics

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/ShreerajShettyK/git_metrics/internal/db"
	"github.com/machinebox/graphql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.mongodb.org/mongo-driver/mongo"
)

// MockGraphQLClient simulates the behavior of the GraphQL client.
type MockGraphQLClient struct {
	mock.Mock
}

func (m *MockGraphQLClient) Run(ctx context.Context, req *graphql.Request, respData interface{}) error {
	args := m.Called(ctx, req, respData)
	if args.Get(0) != nil {
		return args.Error(0)
	}
	return nil
}

// MockHTTPClient simulates the behavior of the HTTP client.
type MockHTTPClient struct {
	mock.Mock
}

func (m *MockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	args := m.Called(req)
	if args.Get(0) != nil {
		return args.Get(0).(*http.Response), args.Error(1)
	}
	return nil, args.Error(1)
}

func TestFetchRepositoriesSimple_Success(t *testing.T) {
	mockClient := new(MockGraphQLClient)
	mockClient.On("Run", mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(2).(*struct {
			User struct {
				Repositories struct {
					Nodes []Repository `json:"nodes"`
				} `json:"repositories"`
			} `json:"user"`
		})
		resp.User.Repositories.Nodes = []Repository{
			{Name: "repo1"},
			{Name: "repo2"},
		}
	})

	repos, err := FetchRepositoriesSimple(mockClient, "user", "token")
	assert.NoError(t, err)
	assert.NotNil(t, repos)
	assert.Len(t, repos, 2)
	assert.Equal(t, "repo1", repos[0].Name)
	assert.Equal(t, "repo2", repos[1].Name)
}

func TestFetchRepositoriesSimple_Error(t *testing.T) {
	mockClient := new(MockGraphQLClient)
	mockClient.On("Run", mock.Anything, mock.Anything, mock.Anything).Return(errors.New("failed to fetch repositories"))

	repos, err := FetchRepositoriesSimple(mockClient, "user", "token")
	assert.Error(t, err)
	assert.Nil(t, repos)
	assert.Contains(t, err.Error(), "failed to fetch repositories")
}

func TestFetchCommits_Error(t *testing.T) {
	mockGraphQLClient := new(MockGraphQLClient)
	mockGraphQLClient.On("Run", mock.Anything, mock.Anything, mock.Anything).Return(errors.New("failed to fetch commits"))

	mockHTTPClient := new(MockHTTPClient)

	commits, err := FetchCommits(mockGraphQLClient, mockHTTPClient, "user", "repo", "token")
	assert.Error(t, err)
	assert.Nil(t, commits)
	assert.Contains(t, err.Error(), "failed to fetch commits")
}

func TestFetchCommits_Success(t *testing.T) {
	mockGraphQLClient := new(MockGraphQLClient)
	mockGraphQLClient.On("Run", mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(2).(*struct {
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
		})
		resp.Repository.Ref.Target.History.Nodes = []struct {
			Oid     string `json:"oid"`
			Message string `json:"message"`
			Author  struct {
				Name string    `json:"name"`
				Date time.Time `json:"date"`
			} `json:"author"`
			Additions    int `json:"additions"`
			Deletions    int `json:"deletions"`
			ChangedFiles int `json:"changedFiles"`
		}{
			{
				Oid:     "commit1",
				Message: "Initial commit",
				Author: struct {
					Name string    `json:"name"`
					Date time.Time `json:"date"`
				}{Name: "author1", Date: time.Now()},
				Additions:    10,
				Deletions:    2,
				ChangedFiles: 5,
			},
		}
	})

	mockHTTPClient := new(MockHTTPClient)
	mockHTTPClient.On("Do", mock.Anything).Return(&http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(`{
			"files": [
				{"status": "added"},
				{"status": "modified"},
				{"status": "removed"}
			]
		}`)),
	}, nil)

	commits, err := FetchCommits(mockGraphQLClient, mockHTTPClient, "user", "repo", "token")
	assert.NoError(t, err)
	assert.NotNil(t, commits)
	assert.Len(t, commits, 1)
	assert.Equal(t, "commit1", commits[0].CommitID)
	assert.Equal(t, 1, commits[0].FilesAdded)
	assert.Equal(t, 1, commits[0].FilesUpdated)
	assert.Equal(t, 1, commits[0].FilesDeleted)
}

func TestFetchCommitFileChanges_Success(t *testing.T) {
	mockHTTPClient := new(MockHTTPClient)
	mockHTTPClient.On("Do", mock.Anything).Return(&http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(`{
			"files": [
				{"status": "added"},
				{"status": "modified"},
				{"status": "removed"}
			]
		}`)),
	}, nil)

	filesAdded, filesDeleted, filesUpdated, err := FetchCommitFileChanges(mockHTTPClient, "user", "repo", "commitID", "token")
	assert.NoError(t, err)
	assert.Equal(t, 1, filesAdded)
	assert.Equal(t, 1, filesUpdated)
	assert.Equal(t, 1, filesDeleted)
}

func TestFetchCommitFileChanges_Error(t *testing.T) {
	mockHTTPClient := new(MockHTTPClient)
	mockHTTPClient.On("Do", mock.Anything).Return(nil, errors.New("failed to fetch commit details"))

	filesAdded, filesDeleted, filesUpdated, err := FetchCommitFileChanges(mockHTTPClient, "user", "repo", "commitID", "token")
	assert.Error(t, err)
	assert.Equal(t, 0, filesAdded)
	assert.Equal(t, 0, filesUpdated)
	assert.Equal(t, 0, filesDeleted)
	assert.Contains(t, err.Error(), "failed to fetch commit details")
}

func TestFetchCommitFileChanges_ErrorCreatingRequest(t *testing.T) {
	mockHTTPClient := new(MockHTTPClient)

	filesAdded, filesDeleted, filesUpdated, err := FetchCommitFileChanges(mockHTTPClient, "user", "repo", "\x00", "token")
	assert.Error(t, err)
	assert.Equal(t, 0, filesAdded)
	assert.Equal(t, 0, filesUpdated)
	assert.Equal(t, 0, filesDeleted)
	assert.Contains(t, err.Error(), "invalid control character in URL")
}

func TestFetchCommitFileChanges_ErrorPerformingRequest(t *testing.T) {
	mockHTTPClient := new(MockHTTPClient)
	mockHTTPClient.On("Do", mock.Anything).Return(nil, errors.New("failed to perform request"))

	filesAdded, filesDeleted, filesUpdated, err := FetchCommitFileChanges(mockHTTPClient, "user", "repo", "commitID", "token")
	assert.Error(t, err)
	assert.Equal(t, 0, filesAdded)
	assert.Equal(t, 0, filesUpdated)
	assert.Equal(t, 0, filesDeleted)
	assert.Contains(t, err.Error(), "failed to perform request")
}

func TestFetchCommitFileChanges_NonOKStatusCode(t *testing.T) {
	mockHTTPClient := new(MockHTTPClient)
	mockHTTPClient.On("Do", mock.Anything).Return(&http.Response{
		StatusCode: http.StatusUnauthorized,
		Body:       io.NopCloser(strings.NewReader("")),
	}, nil)

	filesAdded, filesDeleted, filesUpdated, err := FetchCommitFileChanges(mockHTTPClient, "user", "repo", "commitID", "token")
	assert.Error(t, err)
	assert.Equal(t, 0, filesAdded)
	assert.Equal(t, 0, filesUpdated)
	assert.Equal(t, 0, filesDeleted)
	assert.Contains(t, err.Error(), "failed to fetch commit details")
}

func TestFetchCommitFileChanges_ErrorReadingBody(t *testing.T) {
	mockHTTPClient := new(MockHTTPClient)
	mockHTTPClient.On("Do", mock.Anything).Return(&http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(&errorReader{}),
	}, nil)

	filesAdded, filesDeleted, filesUpdated, err := FetchCommitFileChanges(mockHTTPClient, "user", "repo", "commitID", "token")
	assert.Error(t, err)
	assert.Equal(t, 0, filesAdded)
	assert.Equal(t, 0, filesUpdated)
	assert.Equal(t, 0, filesDeleted)
	assert.Contains(t, err.Error(), "failed to read body")
}

func TestFetchCommitFileChanges_ErrorUnmarshallingBody(t *testing.T) {
	mockHTTPClient := new(MockHTTPClient)
	mockHTTPClient.On("Do", mock.Anything).Return(&http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader("{")),
	}, nil)

	filesAdded, filesDeleted, filesUpdated, err := FetchCommitFileChanges(mockHTTPClient, "user", "repo", "commitID", "token")
	assert.Error(t, err)
	assert.Equal(t, 0, filesAdded)
	assert.Equal(t, 0, filesUpdated)
	assert.Equal(t, 0, filesDeleted)
	assert.Contains(t, err.Error(), "unexpected end of JSON input")
}

type errorReader struct{}

func (*errorReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("failed to read body")
}

func TestSaveCommitsToDB_Success(t *testing.T) {
	commits := []Commit{
		{
			CommitID:      "commit1",
			CommitMessage: "Initial commit",
			LinesDeleted:  2,
			LinesAdded:    10,
			RepoName:      "repo1",
			CommitDate:    time.Now(),
			CommittedBy:   "author1",
			FilesAdded:    1,
			FilesDeleted:  1,
			FilesUpdated:  1,
		},
	}

	mockCollection := new(db.MockCollection)
	mockCollection.On("UpdateOne", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&mongo.UpdateResult{}, nil)

	originalGetCollectionFunc := db.GetCollectionFunc
	defer func() { db.GetCollectionFunc = originalGetCollectionFunc }()
	db.GetCollectionFunc = func() db.CollectionInterface {
		return mockCollection
	}

	err := SaveCommitsToDB(commits)
	assert.NoError(t, err)
	mockCollection.AssertExpectations(t)
}

func TestSaveCommitsToDB_Error(t *testing.T) {
	commits := []Commit{
		{
			CommitID:      "commit1",
			CommitMessage: "Initial commit",
			LinesDeleted:  2,
			LinesAdded:    10,
			RepoName:      "repo1",
			CommitDate:    time.Now(),
			CommittedBy:   "author1",
			FilesAdded:    1,
			FilesDeleted:  1,
			FilesUpdated:  1,
		},
	}

	mockCollection := new(db.MockCollection)
	mockCollection.On("UpdateOne", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(new(mongo.UpdateResult), errors.New("failed to update commit"))

	originalGetCollectionFunc := db.GetCollectionFunc
	defer func() { db.GetCollectionFunc = originalGetCollectionFunc }()
	db.GetCollectionFunc = func() db.CollectionInterface {
		return mockCollection
	}

	err := SaveCommitsToDB(commits)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to update commit")
	mockCollection.AssertExpectations(t)
}
