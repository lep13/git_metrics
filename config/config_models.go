package config

type Config struct {
	GitHubToken string `json:"github_token"`
	MongoDBURI  string `json:"mongodb_uri"`
	Region      string `json:"region"`
}
