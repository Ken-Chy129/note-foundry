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
	"strings"

	"filippo.io/age"
)

type Restorer struct {
	DatabaseURL          string
	AttachmentsDirectory string
	Passphrase           string
	Tools                DatabaseTools
}

type RestoreResult struct {
	Manifest Manifest
}

func (restorer Restorer) Restore(ctx context.Context, source io.Reader) (RestoreResult, error) {
	if strings.TrimSpace(restorer.Passphrase) == "" {
		return RestoreResult{}, ErrPassphraseRequired
	}
	identity, err := age.NewScryptIdentity(restorer.Passphrase)
	if err != nil {
		return RestoreResult{}, fmt.Errorf("configure backup decryption: %w", err)
	}
	decrypted, err := age.Decrypt(source, identity)
	if err != nil {
		return RestoreResult{}, fmt.Errorf("decrypt backup: %w", err)
	}
	gzipReader, err := gzip.NewReader(decrypted)
	if err != nil {
		return RestoreResult{}, fmt.Errorf("open backup compression: %w", err)
	}
	defer gzipReader.Close()

	attachmentsParent := filepath.Dir(restorer.AttachmentsDirectory)
	if err := os.MkdirAll(attachmentsParent, 0o700); err != nil {
		return RestoreResult{}, fmt.Errorf("create Attachments parent: %w", err)
	}
	workspace, err := os.MkdirTemp(attachmentsParent, ".notefoundry-restore-")
	if err != nil {
		return RestoreResult{}, fmt.Errorf("create restore workspace: %w", err)
	}
	defer os.RemoveAll(workspace)
	if err := os.Chmod(workspace, 0o700); err != nil {
		return RestoreResult{}, fmt.Errorf("secure restore workspace: %w", err)
	}

	manifest, dumpPath, restoredAttachments, err := extractArchive(tar.NewReader(gzipReader), workspace)
	if err != nil {
		return RestoreResult{}, err
	}
	if restoredAttachments != manifest.AttachmentCount {
		return RestoreResult{}, fmt.Errorf("%w: manifest declares %d Attachments but archive contains %d", ErrInvalidArchive, manifest.AttachmentCount, restoredAttachments)
	}
	if err := restorer.Tools.Restore(ctx, restorer.DatabaseURL, dumpPath); err != nil {
		return RestoreResult{}, err
	}
	if err := replaceAttachments(restorer.AttachmentsDirectory, filepath.Join(workspace, "attachments"), workspace); err != nil {
		return RestoreResult{}, err
	}
	return RestoreResult{Manifest: manifest}, nil
}

func extractArchive(reader *tar.Reader, workspace string) (Manifest, string, int, error) {
	var manifest Manifest
	var manifestFound bool
	var dumpPath string
	attachmentCount := 0
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return Manifest{}, "", 0, fmt.Errorf("read backup archive: %w", err)
		}
		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA {
			return Manifest{}, "", 0, fmt.Errorf("%w: unsupported entry %q", ErrInvalidArchive, header.Name)
		}
		destination, err := safeArchivePath(workspace, header.Name)
		if err != nil {
			return Manifest{}, "", 0, err
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
			return Manifest{}, "", 0, fmt.Errorf("create restore directory: %w", err)
		}
		if header.Name == "manifest.json" {
			if manifestFound {
				return Manifest{}, "", 0, fmt.Errorf("%w: duplicate manifest", ErrInvalidArchive)
			}
			if err := json.NewDecoder(io.LimitReader(reader, 1<<20)).Decode(&manifest); err != nil {
				return Manifest{}, "", 0, fmt.Errorf("%w: decode manifest: %v", ErrInvalidArchive, err)
			}
			manifestFound = true
			continue
		}
		if header.Name == "database.dump" {
			dumpPath = destination
		} else if strings.HasPrefix(header.Name, "attachments/") {
			attachmentCount++
		} else {
			return Manifest{}, "", 0, fmt.Errorf("%w: unexpected entry %q", ErrInvalidArchive, header.Name)
		}
		file, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			return Manifest{}, "", 0, fmt.Errorf("create restored file: %w", err)
		}
		_, copyErr := io.Copy(file, reader)
		closeErr := file.Close()
		if err := errors.Join(copyErr, closeErr); err != nil {
			return Manifest{}, "", 0, fmt.Errorf("extract %s: %w", header.Name, err)
		}
	}
	if !manifestFound || manifest.FormatVersion != FormatVersion || manifest.DatabaseFormat != "postgres-custom" || dumpPath == "" {
		return Manifest{}, "", 0, ErrInvalidArchive
	}
	if err := os.MkdirAll(filepath.Join(workspace, "attachments"), 0o700); err != nil {
		return Manifest{}, "", 0, fmt.Errorf("create restored Attachments directory: %w", err)
	}
	return manifest, dumpPath, attachmentCount, nil
}

func safeArchivePath(root, name string) (string, error) {
	cleaned := filepath.Clean(filepath.FromSlash(name))
	if cleaned == "." || filepath.IsAbs(cleaned) || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: unsafe entry %q", ErrInvalidArchive, name)
	}
	destination := filepath.Join(root, cleaned)
	relative, err := filepath.Rel(root, destination)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: unsafe entry %q", ErrInvalidArchive, name)
	}
	return destination, nil
}

func replaceAttachments(target, restored, workspace string) error {
	if info, err := os.Lstat(target); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return errors.New("Attachments target cannot be a symbolic link")
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect existing Attachments: %w", err)
	}
	previous := filepath.Join(workspace, "previous-attachments")
	hadPrevious := false
	if _, err := os.Stat(target); err == nil {
		if err := os.Rename(target, previous); err != nil {
			return fmt.Errorf("stage existing Attachments: %w", err)
		}
		hadPrevious = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect existing Attachments: %w", err)
	}
	if err := os.Rename(restored, target); err != nil {
		if hadPrevious {
			_ = os.Rename(previous, target)
		}
		return fmt.Errorf("activate restored Attachments: %w", err)
	}
	if hadPrevious {
		if err := os.RemoveAll(previous); err != nil {
			return fmt.Errorf("remove previous Attachments: %w", err)
		}
	}
	return nil
}
