package backup

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"
)

type PostgresTools struct {
	DumpCommand    string
	RestoreCommand string
}

func (tools PostgresTools) Dump(ctx context.Context, databaseURL, destination string) error {
	commandName := tools.DumpCommand
	if commandName == "" {
		commandName = "pg_dump"
	}
	connection, environment, err := postgresConnection(databaseURL)
	if err != nil {
		return err
	}
	command := exec.CommandContext(ctx, commandName, "--format=custom", "--no-owner", "--no-privileges", "--dbname", connection, "--file", destination)
	command.Env = environment
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("create PostgreSQL dump: %w: %s", err, stderr.String())
	}
	return nil
}

func (tools PostgresTools) Restore(ctx context.Context, databaseURL, source string) error {
	commandName := tools.RestoreCommand
	if commandName == "" {
		commandName = "pg_restore"
	}
	connection, environment, err := postgresConnection(databaseURL)
	if err != nil {
		return err
	}
	command := exec.CommandContext(ctx, commandName, "--clean", "--if-exists", "--no-owner", "--no-privileges", "--dbname", connection, source)
	command.Env = environment
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("restore PostgreSQL dump: %w: %s", err, stderr.String())
	}
	return nil
}

func postgresConnection(databaseURL string) (string, []string, error) {
	parsed, err := url.Parse(databaseURL)
	if err != nil || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") || parsed.Host == "" {
		return "", nil, errors.New("DATABASE_URL must be a PostgreSQL URL")
	}
	var password string
	var hasPassword bool
	if parsed.User != nil {
		password, hasPassword = parsed.User.Password()
		parsed.User = url.User(parsed.User.Username())
	}
	query := parsed.Query()
	if queryPassword := query.Get("password"); queryPassword != "" {
		password = queryPassword
		hasPassword = true
		query.Del("password")
		parsed.RawQuery = query.Encode()
	}
	environment := make([]string, 0, len(os.Environ())+1)
	for _, value := range os.Environ() {
		if !strings.HasPrefix(value, "PGPASSWORD=") {
			environment = append(environment, value)
		}
	}
	if hasPassword {
		environment = append(environment, "PGPASSWORD="+password)
	}
	return parsed.String(), environment, nil
}
