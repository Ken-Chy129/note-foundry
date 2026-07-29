package backup

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Object struct {
	Key          string
	Size         int64
	LastModified time.Time
}

type ObjectStore interface {
	Put(context.Context, string, io.Reader, int64) error
	Get(context.Context, string) (io.ReadCloser, error)
	List(context.Context, string) ([]Object, error)
	Delete(context.Context, string) error
}

type MinioStoreConfig struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	Region    string
	UseSSL    bool
}

type MinioStore struct {
	client *minio.Client
	bucket string
}

func NewMinioStore(config MinioStoreConfig) (*MinioStore, error) {
	client, err := minio.New(config.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.AccessKey, config.SecretKey, ""),
		Secure: config.UseSSL,
		Region: config.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("configure S3-compatible storage: %w", err)
	}
	return &MinioStore{client: client, bucket: config.Bucket}, nil
}

func (store *MinioStore) CheckBucket(ctx context.Context) error {
	exists, err := store.client.BucketExists(ctx, store.bucket)
	if err != nil {
		return fmt.Errorf("check backup bucket: %w", err)
	}
	if !exists {
		return fmt.Errorf("backup bucket %q does not exist", store.bucket)
	}
	return nil
}

func (store *MinioStore) Put(ctx context.Context, key string, source io.Reader, size int64) error {
	_, err := store.client.PutObject(ctx, store.bucket, key, source, size, minio.PutObjectOptions{
		ContentType: "application/octet-stream",
	})
	if err != nil {
		return fmt.Errorf("upload encrypted backup %q: %w", key, err)
	}
	return nil
}

func (store *MinioStore) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	object, err := store.client.GetObject(ctx, store.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("open encrypted backup %q: %w", key, err)
	}
	if _, err := object.Stat(); err != nil {
		object.Close()
		return nil, fmt.Errorf("inspect encrypted backup %q: %w", key, err)
	}
	return object, nil
}

func (store *MinioStore) List(ctx context.Context, prefix string) ([]Object, error) {
	var objects []Object
	for object := range store.client.ListObjects(ctx, store.bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true}) {
		if object.Err != nil {
			return nil, fmt.Errorf("list encrypted backups: %w", object.Err)
		}
		objects = append(objects, Object{Key: object.Key, Size: object.Size, LastModified: object.LastModified})
	}
	sort.Slice(objects, func(left, right int) bool {
		if objects[left].LastModified.Equal(objects[right].LastModified) {
			return strings.Compare(objects[left].Key, objects[right].Key) > 0
		}
		return objects[left].LastModified.After(objects[right].LastModified)
	})
	return objects, nil
}

func (store *MinioStore) Delete(ctx context.Context, key string) error {
	if err := store.client.RemoveObject(ctx, store.bucket, key, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("delete expired backup %q: %w", key, err)
	}
	return nil
}
