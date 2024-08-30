package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lep13/git_metrics/config"
	"github.com/lep13/git_metrics/internal/db"
	"github.com/lep13/git_metrics/internal/gitmetrics"
	"github.com/stretchr/testify/assert"
)

func TestStartServer(t *testing.T) {
	// real LoadConfig and InitializeMongoDB for this test
	LoadConfigFunc = config.LoadConfig
	InitializeMongoDBFunc = db.InitializeMongoDB

	// Create a real instance of GitMetrics
	gitMetrics := &gitmetrics.GitMetricsImpl{}

	// Create a new ServeMux for testing
	mux := http.NewServeMux()

	// Start the server in a goroutine
	go func() {
		StartServer(mux, gitMetrics)
	}()

	// Allow some time for the server to start
	time.Sleep(2 * time.Second)

	// Create a test server using httptest
	ts := httptest.NewServer(mux)
	defer ts.Close()

	// Create a request to pass to our handler
	req, err := http.NewRequest("GET", ts.URL+"/commits?user=test_user", nil)
	assert.NoError(t, err)

	// Perform the request
	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	defer resp.Body.Close()

	// Check the status code is what we expect
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Read and check the response body
	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.Equal(t, "Commits fetched and stored in MongoDB successfully.", string(body))
}
