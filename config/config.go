package config

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

type Config struct {
	GitHubToken string `json:"github_token"`
	MongoDBURI  string `json:"mongodb_uri"`
	Region      string `json:"region"`
}

func LoadConfig() (*Config, error) {
	secretName := "git_metrics"

	// Load the Shared AWS Configuration (~/.aws/config)
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, fmt.Errorf("unable to load SDK config, %v", err)
	}

	// Create a Secrets Manager client
	svc := secretsmanager.NewFromConfig(cfg)

	input := &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretName),
	}

	result, err := svc.GetSecretValue(context.TODO(), input)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve secret: %w", err)
	}

	secretString := *result.SecretString
	config := &Config{}

	err = json.Unmarshal([]byte(secretString), config)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal secret string: %w", err)
	}

	return config, nil
}
