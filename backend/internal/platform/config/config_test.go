package config

import (
	"errors"
	"testing"
)

func TestLoadUsesDevelopmentDefaults(t *testing.T) {
	config, err := Load(mapLookup(map[string]string{
		"DATABASE_URL": "postgres://notefoundry@localhost/notefoundry",
	}))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if config.Environment != EnvironmentDevelopment {
		t.Errorf("Environment = %q, want %q", config.Environment, EnvironmentDevelopment)
	}
	if config.HTTPAddress != ":8080" {
		t.Errorf("HTTPAddress = %q, want %q", config.HTTPAddress, ":8080")
	}
	if config.DatabaseURL != "postgres://notefoundry@localhost/notefoundry" {
		t.Errorf("DatabaseURL = %q", config.DatabaseURL)
	}
}

func TestLoadRejectsMissingDatabaseURL(t *testing.T) {
	_, err := Load(mapLookup(nil))
	if !errors.Is(err, ErrDatabaseURLRequired) {
		t.Fatalf("Load() error = %v, want %v", err, ErrDatabaseURLRequired)
	}
}

func TestLoadRejectsUnknownEnvironment(t *testing.T) {
	_, err := Load(mapLookup(map[string]string{
		"APP_ENV":      "staging-ish",
		"DATABASE_URL": "postgres://notefoundry@localhost/notefoundry",
	}))
	if !errors.Is(err, ErrInvalidEnvironment) {
		t.Fatalf("Load() error = %v, want %v", err, ErrInvalidEnvironment)
	}
}

func TestLoadTrimsConfiguredValues(t *testing.T) {
	config, err := Load(mapLookup(map[string]string{
		"APP_ENV":      " production ",
		"HTTP_ADDR":    " 127.0.0.1:9090 ",
		"DATABASE_URL": " postgres://notefoundry@db/notefoundry ",
	}))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if config.Environment != EnvironmentProduction {
		t.Errorf("Environment = %q, want %q", config.Environment, EnvironmentProduction)
	}
	if config.HTTPAddress != "127.0.0.1:9090" {
		t.Errorf("HTTPAddress = %q, want %q", config.HTTPAddress, "127.0.0.1:9090")
	}
	if config.DatabaseURL != "postgres://notefoundry@db/notefoundry" {
		t.Errorf("DatabaseURL = %q", config.DatabaseURL)
	}
}

func mapLookup(values map[string]string) LookupEnv {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}
