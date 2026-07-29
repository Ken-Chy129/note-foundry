package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"filippo.io/age"
)

const FormatVersion = 1

var (
	ErrPassphraseRequired = errors.New("backup passphrase is required")
	ErrInvalidArchive     = errors.New("backup archive is invalid")
)

type Manifest struct {
	FormatVersion   int       `json:"formatVersion"`
	CreatedAt       time.Time `json:"createdAt"`
	DatabaseFormat  string    `json:"databaseFormat"`
	AttachmentCount int       `json:"attachmentCount"`
}

type DatabaseTools interface {
	Dump(context.Context, string, string) error
	Restore(context.Context, string, string) error
}

type Archiver struct {
	DatabaseURL          string
	AttachmentsDirectory string
	Passphrase           string
	Tools                DatabaseTools
	TemporaryDirectory   string
}

func (archiver Archiver) Create(ctx context.Context, destination io.Writer, createdAt time.Time) (Manifest, error) {
	if strings.TrimSpace(archiver.Passphrase) == "" {
		return Manifest{}, ErrPassphraseRequired
	}
	temporaryDirectory, err := os.MkdirTemp(archiver.TemporaryDirectory, "notefoundry-backup-")
	if err != nil {
		return Manifest{}, fmt.Errorf("create backup workspace: %w", err)
	}
	defer os.RemoveAll(temporaryDirectory)
	if err := os.Chmod(temporaryDirectory, 0o700); err != nil {
		return Manifest{}, fmt.Errorf("secure backup workspace: %w", err)
	}

	dumpPath := filepath.Join(temporaryDirectory, "database.dump")
	if err := archiver.Tools.Dump(ctx, archiver.DatabaseURL, dumpPath); err != nil {
		return Manifest{}, err
	}
	if err := os.Chmod(dumpPath, 0o600); err != nil {
		return Manifest{}, fmt.Errorf("secure database dump: %w", err)
	}
	attachments, err := attachmentFiles(archiver.AttachmentsDirectory)
	if err != nil {
		return Manifest{}, err
	}
	manifest := Manifest{
		FormatVersion:   FormatVersion,
		CreatedAt:       createdAt.UTC(),
		DatabaseFormat:  "postgres-custom",
		AttachmentCount: len(attachments),
	}

	recipient, err := age.NewScryptRecipient(archiver.Passphrase)
	if err != nil {
		return Manifest{}, fmt.Errorf("configure backup encryption: %w", err)
	}
	encryptedWriter, err := age.Encrypt(destination, recipient)
	if err != nil {
		return Manifest{}, fmt.Errorf("start backup encryption: %w", err)
	}
	gzipWriter := gzip.NewWriter(encryptedWriter)
	tarWriter := tar.NewWriter(gzipWriter)

	writeErr := writeArchive(tarWriter, manifest, dumpPath, archiver.AttachmentsDirectory, attachments)
	closeErr := errors.Join(tarWriter.Close(), gzipWriter.Close(), encryptedWriter.Close())
	if err := errors.Join(writeErr, closeErr); err != nil {
		return Manifest{}, fmt.Errorf("write encrypted backup: %w", err)
	}
	return manifest, nil
}

func writeArchive(writer *tar.Writer, manifest Manifest, dumpPath, attachmentsDirectory string, attachments []string) error {
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("encode backup manifest: %w", err)
	}
	if err := writeBytes(writer, "manifest.json", manifestJSON, 0o600, manifest.CreatedAt); err != nil {
		return err
	}
	if err := writeFile(writer, "database.dump", dumpPath, 0o600, manifest.CreatedAt); err != nil {
		return err
	}
	for _, relativePath := range attachments {
		archivePath := filepath.ToSlash(filepath.Join("attachments", relativePath))
		if err := writeFile(writer, archivePath, filepath.Join(attachmentsDirectory, relativePath), 0o600, manifest.CreatedAt); err != nil {
			return err
		}
	}
	return nil
}

func writeBytes(writer *tar.Writer, name string, contents []byte, mode int64, modifiedAt time.Time) error {
	header := &tar.Header{Name: name, Mode: mode, Size: int64(len(contents)), ModTime: modifiedAt}
	if err := writer.WriteHeader(header); err != nil {
		return fmt.Errorf("write %s header: %w", name, err)
	}
	if _, err := writer.Write(contents); err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}
	return nil
}

func writeFile(writer *tar.Writer, name, path string, mode int64, modifiedAt time.Time) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open %s: %w", name, err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("inspect %s: %w", name, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", name)
	}
	header := &tar.Header{Name: name, Mode: mode, Size: info.Size(), ModTime: modifiedAt}
	if err := writer.WriteHeader(header); err != nil {
		return fmt.Errorf("write %s header: %w", name, err)
	}
	if _, err := io.Copy(writer, file); err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}
	return nil
}

func attachmentFiles(root string) ([]string, error) {
	if _, err := os.Stat(root); errors.Is(err, os.ErrNotExist) {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("inspect Attachments directory: %w", err)
	}
	var files []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root || entry.IsDir() {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("Attachment %s is a symbolic link", path)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("Attachment %s is not a regular file", path)
		}
		relativePath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, relativePath)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("enumerate Attachments: %w", err)
	}
	sort.Strings(files)
	return files, nil
}
