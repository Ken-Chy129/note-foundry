package backup

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
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
	command := exec.CommandContext(ctx, commandName, "--format=custom", "--no-owner", "--no-privileges", "--file", destination)
	command.Env = append(os.Environ(), "PGDATABASE="+databaseURL)
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
	command := exec.CommandContext(ctx, commandName, "--clean", "--if-exists", "--no-owner", "--no-privileges", "--dbname=", source)
	command.Env = append(os.Environ(), "PGDATABASE="+databaseURL)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("restore PostgreSQL dump: %w: %s", err, stderr.String())
	}
	return nil
}
