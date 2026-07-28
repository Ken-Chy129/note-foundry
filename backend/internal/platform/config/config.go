package config

import (
	"errors"
	"strings"
)

var (
	ErrDatabaseURLRequired = errors.New("DATABASE_URL is required")
	ErrInvalidEnvironment  = errors.New("APP_ENV must be development, test, or production")
)

type Environment string

const (
	EnvironmentDevelopment Environment = "development"
	EnvironmentTest        Environment = "test"
	EnvironmentProduction  Environment = "production"
)

type Config struct {
	Environment Environment
	HTTPAddress string
	DatabaseURL string
}

type LookupEnv func(string) (string, bool)

func Load(lookup LookupEnv) (Config, error) {
	environment := Environment(valueOrDefault(lookup, "APP_ENV", string(EnvironmentDevelopment)))
	if environment != EnvironmentDevelopment && environment != EnvironmentTest && environment != EnvironmentProduction {
		return Config{}, ErrInvalidEnvironment
	}

	databaseURL := valueOrDefault(lookup, "DATABASE_URL", "")
	if databaseURL == "" {
		return Config{}, ErrDatabaseURLRequired
	}

	return Config{
		Environment: environment,
		HTTPAddress: valueOrDefault(lookup, "HTTP_ADDR", ":8080"),
		DatabaseURL: databaseURL,
	}, nil
}

func valueOrDefault(lookup LookupEnv, key, fallback string) string {
	value, ok := lookup(key)
	value = strings.TrimSpace(value)
	if !ok || value == "" {
		return fallback
	}
	return value
}
