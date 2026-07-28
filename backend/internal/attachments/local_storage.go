package attachments

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
)

var (
	ErrInvalidStorageKey = errors.New("invalid attachment storage key")
	storageKeyPattern    = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
)

type StoredObject struct {
	SizeBytes int64
	SHA256Hex string
}

type LocalStorage struct {
	root string
}

func NewLocalStorage(root string) (*LocalStorage, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve attachment storage directory: %w", err)
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, fmt.Errorf("create attachment storage directory: %w", err)
	}
	return &LocalStorage{root: root}, nil
}

func (storage *LocalStorage) Put(ctx context.Context, key string, reader io.Reader) (StoredObject, error) {
	path, err := storage.pathForKey(key)
	if err != nil {
		return StoredObject{}, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return StoredObject{}, fmt.Errorf("create attachment shard directory: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".upload-*")
	if err != nil {
		return StoredObject{}, fmt.Errorf("create temporary attachment: %w", err)
	}
	temporaryPath := temporary.Name()
	defer func() {
		_ = temporary.Close()
		_ = os.Remove(temporaryPath)
	}()
	if err := temporary.Chmod(0o600); err != nil {
		return StoredObject{}, fmt.Errorf("protect temporary attachment: %w", err)
	}
	hash := sha256.New()
	written, err := copyWithContext(ctx, io.MultiWriter(temporary, hash), reader)
	if err != nil {
		return StoredObject{}, fmt.Errorf("write attachment: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return StoredObject{}, fmt.Errorf("sync attachment: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return StoredObject{}, fmt.Errorf("close attachment: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return StoredObject{}, fmt.Errorf("commit attachment: %w", err)
	}
	return StoredObject{SizeBytes: written, SHA256Hex: hex.EncodeToString(hash.Sum(nil))}, nil
}

func (storage *LocalStorage) Open(_ context.Context, key string) (ReadSeekCloser, error) {
	path, err := storage.pathForKey(key)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open attachment: %w", err)
	}
	return file, nil
}

func (storage *LocalStorage) Delete(_ context.Context, key string) error {
	path, err := storage.pathForKey(key)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("delete attachment: %w", err)
	}
	return nil
}

func (storage *LocalStorage) pathForKey(key string) (string, error) {
	if !storageKeyPattern.MatchString(key) {
		return "", ErrInvalidStorageKey
	}
	return filepath.Join(storage.root, key[:2], key), nil
}

func copyWithContext(ctx context.Context, destination io.Writer, source io.Reader) (int64, error) {
	buffer := make([]byte, 32*1024)
	var written int64
	for {
		if err := ctx.Err(); err != nil {
			return written, err
		}
		count, readErr := source.Read(buffer)
		if count > 0 {
			writeCount, writeErr := destination.Write(buffer[:count])
			written += int64(writeCount)
			if writeErr != nil {
				return written, writeErr
			}
			if writeCount != count {
				return written, io.ErrShortWrite
			}
		}
		if errors.Is(readErr, io.EOF) {
			return written, nil
		}
		if readErr != nil {
			return written, readErr
		}
	}
}
