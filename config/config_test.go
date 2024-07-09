package config

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/stretchr/testify/assert"
)

// MockSecretsManager simulates the behavior of the Secrets Manager client.
type MockSecretsManager struct {
	GetSecretValueFunc func(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
}

func (m *MockSecretsManager) GetSecretValue(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
	return m.GetSecretValueFunc(ctx, params, optFns...)
}

func mockLoadAWSConfig(ctx context.Context, optFns ...func(*config.LoadOptions) error) (aws.Config, error) {
	return aws.Config{Region: "us-east-1"}, nil
}

func TestLoadConfig_Success(t *testing.T) {
	// Mock the AWS configuration loading function
	originalLoadAWSConfig := loadAWSConfig
	defer func() { loadAWSConfig = originalLoadAWSConfig }()

	loadAWSConfig = mockLoadAWSConfig

	// Mock Secrets Manager
	mockSM := &MockSecretsManager{
		GetSecretValueFunc: func(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
			secretString := `{"github_token":"mock_token","mongodb_uri":"mock_uri","region":"mock_region"}`
			return &secretsmanager.GetSecretValueOutput{
				SecretString: aws.String(secretString),
			}, nil
		},
	}

	// Override SecretManagerFunc to return the mock Secrets Manager
	originalSecretManagerFunc := SecretManagerFunc
	defer func() { SecretManagerFunc = originalSecretManagerFunc }()
	SecretManagerFunc = func() (SecretsManagerInterface, error) {
		return mockSM, nil
	}

	config, err := LoadConfig()
	assert.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, "mock_token", config.GitHubToken)
	assert.Equal(t, "mock_uri", config.MongoDBURI)
	assert.Equal(t, "mock_region", config.Region)
}

func TestLoadConfig_SecretsManagerError(t *testing.T) {
	// Mock the AWS configuration loading function
	originalLoadAWSConfig := loadAWSConfig
	defer func() { loadAWSConfig = originalLoadAWSConfig }()

	loadAWSConfig = mockLoadAWSConfig

	// Mock Secrets Manager to simulate an error
	mockSM := &MockSecretsManager{
		GetSecretValueFunc: func(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
			return nil, errors.New("failed to retrieve secret")
		},
	}

	// Override SecretManagerFunc to return the mock Secrets Manager
	originalSecretManagerFunc := SecretManagerFunc
	defer func() { SecretManagerFunc = originalSecretManagerFunc }()
	SecretManagerFunc = func() (SecretsManagerInterface, error) {
		return mockSM, nil
	}

	config, err := LoadConfig()
	assert.Error(t, err)
	assert.Nil(t, config)
	assert.Contains(t, err.Error(), "failed to retrieve secret")
}

func TestLoadConfig_UnmarshalError(t *testing.T) {
	// Mock the AWS configuration loading function
	originalLoadAWSConfig := loadAWSConfig
	defer func() { loadAWSConfig = originalLoadAWSConfig }()

	loadAWSConfig = mockLoadAWSConfig

	// Mock Secrets Manager to return invalid JSON
	mockSM := &MockSecretsManager{
		GetSecretValueFunc: func(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
			secretString := `invalid_json`
			return &secretsmanager.GetSecretValueOutput{
				SecretString: aws.String(secretString),
			}, nil
		},
	}

	// Override SecretManagerFunc to return the mock Secrets Manager
	originalSecretManagerFunc := SecretManagerFunc
	defer func() { SecretManagerFunc = originalSecretManagerFunc }()
	SecretManagerFunc = func() (SecretsManagerInterface, error) {
		return mockSM, nil
	}

	config, err := LoadConfig()
	assert.Error(t, err)
	assert.Nil(t, config)
	assert.Contains(t, err.Error(), "failed to unmarshal secret string")
}

func TestLoadConfig_AWSConfigLoadingFailure(t *testing.T) {
	// Mock the AWS configuration loading function to simulate failure
	originalLoadAWSConfig := loadAWSConfig
	defer func() { loadAWSConfig = originalLoadAWSConfig }()

	loadAWSConfig = func(ctx context.Context, optFns ...func(*config.LoadOptions) error) (aws.Config, error) {
		return aws.Config{}, errors.New("failed to load AWS config")
	}

	// Override SecretManagerFunc to return an error when AWS config fails
	originalSecretManagerFunc := SecretManagerFunc
	defer func() { SecretManagerFunc = originalSecretManagerFunc }()
	SecretManagerFunc = func() (SecretsManagerInterface, error) {
		return nil, errors.New("failed to load AWS config")
	}

	_, err := LoadConfig()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load AWS config")
}

func TestSecretManagerFunc(t *testing.T) {
	// Test case for successful AWS config loading
	t.Run("Successful AWS Config Loading", func(t *testing.T) {
		// Mock the AWS configuration loading function to simulate success
		originalLoadAWSConfig := loadAWSConfig
		defer func() { loadAWSConfig = originalLoadAWSConfig }()

		loadAWSConfig = mockLoadAWSConfig

		svc, err := SecretManagerFunc()
		assert.NoError(t, err)
		assert.NotNil(t, svc)
	})

	// Test case for AWS config loading failure
	t.Run("AWS Config Loading Failure", func(t *testing.T) {
		// Mock the AWS configuration loading function to simulate failure
		originalLoadAWSConfig := loadAWSConfig
		defer func() { loadAWSConfig = originalLoadAWSConfig }()

		loadAWSConfig = func(ctx context.Context, optFns ...func(*config.LoadOptions) error) (aws.Config, error) {
			return aws.Config{}, errors.New("failed to load AWS config")
		}

		svc, err := SecretManagerFunc()
		assert.Error(t, err)
		assert.Nil(t, svc)
		assert.Contains(t, err.Error(), "failed to load AWS config")
	})
}
