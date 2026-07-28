package config

import (
	"errors"
	"testing"
)

func TestLoadUsesDevelopmentDefaults(t *testing.T) {
	config, err := Load(mapLookup(validConfigValues()))
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
	if config.SecureCookies {
		t.Error("SecureCookies = true in development, want false")
	}
	if config.AttachmentsDirectory != "./data/attachments" {
		t.Errorf("AttachmentsDirectory = %q", config.AttachmentsDirectory)
	}
}

func TestLoadRejectsMissingDatabaseURL(t *testing.T) {
	_, err := Load(mapLookup(nil))
	if !errors.Is(err, ErrDatabaseURLRequired) {
		t.Fatalf("Load() error = %v, want %v", err, ErrDatabaseURLRequired)
	}
}

func TestLoadRejectsUnknownEnvironment(t *testing.T) {
	values := validConfigValues()
	values["APP_ENV"] = "staging-ish"
	_, err := Load(mapLookup(values))
	if !errors.Is(err, ErrInvalidEnvironment) {
		t.Fatalf("Load() error = %v, want %v", err, ErrInvalidEnvironment)
	}
}

func TestLoadTrimsConfiguredValues(t *testing.T) {
	values := validConfigValues()
	values["APP_ENV"] = " production "
	values["HTTP_ADDR"] = " 127.0.0.1:9090 "
	values["DATABASE_URL"] = " postgres://notefoundry@db/notefoundry "
	values["PUBLIC_URL"] = " https://notes.test/ "
	values["ATTACHMENTS_DIR"] = " /srv/notefoundry/attachments "
	config, err := Load(mapLookup(values))
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
	if config.PublicURL != "https://notes.test" {
		t.Errorf("PublicURL = %q", config.PublicURL)
	}
	if !config.SecureCookies {
		t.Error("SecureCookies = false in production, want true")
	}
	if config.AttachmentsDirectory != "/srv/notefoundry/attachments" {
		t.Errorf("AttachmentsDirectory = %q", config.AttachmentsDirectory)
	}
}

func TestLoadRejectsMissingGitHubOwnerConfiguration(t *testing.T) {
	values := validConfigValues()
	delete(values, "GITHUB_OWNER_ID")

	_, err := Load(mapLookup(values))
	if !errors.Is(err, ErrGitHubOwnerIDRequired) {
		t.Fatalf("Load() error = %v, want %v", err, ErrGitHubOwnerIDRequired)
	}
}

func TestLoadRequiresHTTPSPublicURLInProduction(t *testing.T) {
	values := validConfigValues()
	values["APP_ENV"] = "production"

	_, err := Load(mapLookup(values))
	if !errors.Is(err, ErrProductionHTTPSRequired) {
		t.Fatalf("Load() error = %v, want %v", err, ErrProductionHTTPSRequired)
	}
}

func mapLookup(values map[string]string) LookupEnv {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}

func validConfigValues() map[string]string {
	return map[string]string{
		"DATABASE_URL":         "postgres://notefoundry@localhost/notefoundry",
		"PUBLIC_URL":           "http://localhost:3000",
		"GITHUB_CLIENT_ID":     "github-client-id",
		"GITHUB_CLIENT_SECRET": "github-client-secret",
		"GITHUB_OWNER_ID":      "42",
	}
}
