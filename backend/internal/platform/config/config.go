package config

import (
	"errors"
	"net/url"
	"strconv"
	"strings"
)

var (
	ErrDatabaseURLRequired        = errors.New("DATABASE_URL is required")
	ErrInvalidEnvironment         = errors.New("APP_ENV must be development, test, or production")
	ErrPublicURLRequired          = errors.New("PUBLIC_URL is required")
	ErrInvalidPublicURL           = errors.New("PUBLIC_URL must be an absolute HTTP or HTTPS URL")
	ErrProductionHTTPSRequired    = errors.New("PUBLIC_URL must use HTTPS in production")
	ErrGitHubClientIDRequired     = errors.New("GITHUB_CLIENT_ID is required")
	ErrGitHubClientSecretRequired = errors.New("GITHUB_CLIENT_SECRET is required")
	ErrGitHubOwnerIDRequired      = errors.New("GITHUB_OWNER_ID must be a positive integer")
)

type Environment string

const (
	EnvironmentDevelopment Environment = "development"
	EnvironmentTest        Environment = "test"
	EnvironmentProduction  Environment = "production"
)

type Config struct {
	Environment          Environment
	HTTPAddress          string
	DatabaseURL          string
	PublicURL            string
	GitHubClientID       string
	GitHubClientSecret   string
	GitHubOwnerID        int64
	SecureCookies        bool
	AttachmentsDirectory string
}

type LookupEnv func(string) (string, bool)

func Load(lookup LookupEnv) (Config, error) {
	environment := Environment(valueOrDefault(lookup, "APP_ENV", string(EnvironmentDevelopment)))
	if environment != EnvironmentDevelopment && environment != EnvironmentTest && environment != EnvironmentProduction {
		return Config{}, ErrInvalidEnvironment
	}

	databaseURL, err := LoadDatabaseURL(lookup)
	if err != nil {
		return Config{}, err
	}
	publicURL := strings.TrimRight(valueOrDefault(lookup, "PUBLIC_URL", ""), "/")
	if publicURL == "" {
		return Config{}, ErrPublicURLRequired
	}
	parsedPublicURL, err := url.Parse(publicURL)
	if err != nil || parsedPublicURL.Host == "" || (parsedPublicURL.Scheme != "http" && parsedPublicURL.Scheme != "https") {
		return Config{}, ErrInvalidPublicURL
	}
	if environment == EnvironmentProduction && parsedPublicURL.Scheme != "https" {
		return Config{}, ErrProductionHTTPSRequired
	}

	githubClientID := valueOrDefault(lookup, "GITHUB_CLIENT_ID", "")
	if githubClientID == "" {
		return Config{}, ErrGitHubClientIDRequired
	}
	githubClientSecret := valueOrDefault(lookup, "GITHUB_CLIENT_SECRET", "")
	if githubClientSecret == "" {
		return Config{}, ErrGitHubClientSecretRequired
	}
	githubOwnerID, err := strconv.ParseInt(valueOrDefault(lookup, "GITHUB_OWNER_ID", ""), 10, 64)
	if err != nil || githubOwnerID <= 0 {
		return Config{}, ErrGitHubOwnerIDRequired
	}

	return Config{
		Environment:          environment,
		HTTPAddress:          valueOrDefault(lookup, "HTTP_ADDR", ":8080"),
		DatabaseURL:          databaseURL,
		PublicURL:            publicURL,
		GitHubClientID:       githubClientID,
		GitHubClientSecret:   githubClientSecret,
		GitHubOwnerID:        githubOwnerID,
		SecureCookies:        environment == EnvironmentProduction,
		AttachmentsDirectory: valueOrDefault(lookup, "ATTACHMENTS_DIR", "./data/attachments"),
	}, nil
}

func LoadDatabaseURL(lookup LookupEnv) (string, error) {
	databaseURL := valueOrDefault(lookup, "DATABASE_URL", "")
	if databaseURL == "" {
		return "", ErrDatabaseURLRequired
	}
	return databaseURL, nil
}

func valueOrDefault(lookup LookupEnv, key, fallback string) string {
	value, ok := lookup(key)
	value = strings.TrimSpace(value)
	if !ok || value == "" {
		return fallback
	}
	return value
}
