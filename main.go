package main

import (
	"net/http"

	"github.com/ShreerajShettyK/git_metrics/internal/gitmetrics"
	"github.com/ShreerajShettyK/git_metrics/server"
)

func main() {
	mux := http.NewServeMux()
	gitMetrics := &gitmetrics.GitMetricsImpl{}
	server.StartServer(mux, gitMetrics)
}
