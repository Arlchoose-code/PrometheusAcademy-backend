package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"academyprometheus/backend/config"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var ErrObjectNotFound = errors.New("storage object not found")

type PutObjectInput struct {
	Key, ContentType, CacheControl string
	Body                           io.Reader
}

type StoredObject struct {
	Key, Provider, Bucket, ChecksumSHA256 string
	Size                                  int64
}
type ObjectInfo struct {
	Key, ContentType, ChecksumSHA256 string
	Size                             int64
	ModifiedAt                       time.Time
}

type ObjectStorage interface {
	Put(context.Context, PutObjectInput) (StoredObject, error)
	Open(context.Context, string) (io.ReadCloser, ObjectInfo, error)
	Exists(context.Context, string) (bool, error)
	Delete(context.Context, string) error
	Copy(context.Context, string, string) error
	SignedURL(context.Context, string, time.Duration) (string, error)
}

type LocalStorage struct{ Root string }

func (s *LocalStorage) path(key string) (string, error) {
	key = strings.TrimLeft(filepath.ToSlash(key), "/")
	clean := filepath.Clean(filepath.FromSlash(key))
	if clean == "." || strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
		return "", fmt.Errorf("invalid object key")
	}
	root, _ := filepath.Abs(s.Root)
	path, _ := filepath.Abs(filepath.Join(root, clean))
	if path != root && !strings.HasPrefix(path, root+string(filepath.Separator)) {
		return "", fmt.Errorf("object key escapes storage root")
	}
	return path, nil
}
func (s *LocalStorage) Put(ctx context.Context, in PutObjectInput) (StoredObject, error) {
	p, err := s.path(in.Key)
	if err != nil {
		return StoredObject{}, err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return StoredObject{}, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(p), ".object-*")
	if err != nil {
		return StoredObject{}, err
	}
	defer os.Remove(tmp.Name())
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(tmp, h), &contextReader{ctx: ctx, r: in.Body})
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return StoredObject{}, err
	}
	if err = os.Rename(tmp.Name(), p); err != nil {
		return StoredObject{}, err
	}
	return StoredObject{Key: in.Key, Provider: "local", Size: n, ChecksumSHA256: hex.EncodeToString(h.Sum(nil))}, nil
}
func (s *LocalStorage) Open(_ context.Context, key string) (io.ReadCloser, ObjectInfo, error) {
	p, e := s.path(key)
	if e != nil {
		return nil, ObjectInfo{}, e
	}
	f, e := os.Open(p)
	if os.IsNotExist(e) {
		return nil, ObjectInfo{}, ErrObjectNotFound
	}
	if e != nil {
		return nil, ObjectInfo{}, e
	}
	st, e := f.Stat()
	if e != nil {
		f.Close()
		return nil, ObjectInfo{}, e
	}
	return f, ObjectInfo{Key: key, Size: st.Size(), ModifiedAt: st.ModTime()}, nil
}
func (s *LocalStorage) Exists(ctx context.Context, key string) (bool, error) {
	r, _, e := s.Open(ctx, key)
	if e == ErrObjectNotFound {
		return false, nil
	}
	if e != nil {
		return false, e
	}
	r.Close()
	return true, nil
}
func (s *LocalStorage) Delete(_ context.Context, key string) error {
	p, e := s.path(key)
	if e != nil {
		return e
	}
	e = os.Remove(p)
	if os.IsNotExist(e) {
		return nil
	}
	return e
}
func (s *LocalStorage) Copy(ctx context.Context, a, b string) error {
	r, _, e := s.Open(ctx, a)
	if e != nil {
		return e
	}
	defer r.Close()
	_, e = s.Put(ctx, PutObjectInput{Key: b, Body: r})
	return e
}
func (s *LocalStorage) SignedURL(_ context.Context, _ string, _ time.Duration) (string, error) {
	return "", errors.New("local storage does not issue signed URLs")
}

type R2Storage struct {
	client  *s3.Client
	presign *s3.PresignClient
	bucket  string
}

func NewR2Storage(ctx context.Context, c config.Config, bucket, access, secret string) (*R2Storage, error) {
	if c.R2AccountID == "" || bucket == "" || access == "" || secret == "" {
		return nil, errors.New("R2 is not configured")
	}
	endpoint := "https://" + c.R2AccountID + ".r2.cloudflarestorage.com"
	ac, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion("auto"), awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(access, secret, "")))
	if err != nil {
		return nil, err
	}
	cl := s3.NewFromConfig(ac, func(o *s3.Options) { o.BaseEndpoint = aws.String(endpoint); o.UsePathStyle = true })
	return &R2Storage{client: cl, presign: s3.NewPresignClient(cl), bucket: bucket}, nil
}
func (s *R2Storage) Put(ctx context.Context, in PutObjectInput) (StoredObject, error) {
	h := sha256.New()
	out, err := s.client.PutObject(ctx, &s3.PutObjectInput{Bucket: &s.bucket, Key: &in.Key, Body: io.TeeReader(in.Body, h), ContentType: optionalString(in.ContentType), CacheControl: optionalString(in.CacheControl)})
	if err != nil {
		return StoredObject{}, err
	}
	_ = out
	head, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: &s.bucket, Key: &in.Key})
	if err != nil {
		return StoredObject{}, err
	}
	return StoredObject{Key: in.Key, Provider: "r2", Bucket: s.bucket, Size: aws.ToInt64(head.ContentLength), ChecksumSHA256: hex.EncodeToString(h.Sum(nil))}, nil
}
func (s *R2Storage) Open(ctx context.Context, key string) (io.ReadCloser, ObjectInfo, error) {
	o, e := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: &s.bucket, Key: &key})
	if e != nil {
		return nil, ObjectInfo{}, e
	}
	return o.Body, ObjectInfo{Key: key, Size: aws.ToInt64(o.ContentLength), ContentType: aws.ToString(o.ContentType), ModifiedAt: aws.ToTime(o.LastModified)}, nil
}
func (s *R2Storage) Exists(ctx context.Context, key string) (bool, error) {
	_, e := s.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: &s.bucket, Key: &key})
	if e != nil {
		return false, nil
	}
	return true, nil
}
func (s *R2Storage) Delete(ctx context.Context, key string) error {
	_, e := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: &s.bucket, Key: &key})
	return e
}
func (s *R2Storage) Copy(ctx context.Context, a, b string) error {
	src := s.bucket + "/" + a
	_, e := s.client.CopyObject(ctx, &s3.CopyObjectInput{Bucket: &s.bucket, Key: &b, CopySource: &src})
	return e
}
func (s *R2Storage) SignedURL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	o, e := s.presign.PresignGetObject(ctx, &s3.GetObjectInput{Bucket: &s.bucket, Key: &key}, s3.WithPresignExpires(ttl))
	if e != nil {
		return "", e
	}
	return o.URL, nil
}
func optionalString(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r *contextReader) Read(p []byte) (int, error) {
	select {
	case <-r.ctx.Done():
		return 0, r.ctx.Err()
	default:
		return r.r.Read(p)
	}
}

func NewObjectStorage(ctx context.Context, c config.Config) (ObjectStorage, error) {
	if c.StorageProvider == "r2" {
		return NewR2Storage(ctx, c, c.R2Bucket, c.R2AccessKeyID, c.R2SecretAccessKey)
	}
	return &LocalStorage{Root: c.StoragePath}, nil
}
