package services

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func TestLocalStorageContract(t *testing.T) {
	s := &LocalStorage{Root: t.TempDir()}
	ctx := context.Background()
	body := []byte("immutable-object")
	got, err := s.Put(ctx, PutObjectInput{Key: "protected/test/a.bin", Body: bytes.NewReader(body)})
	if err != nil {
		t.Fatal(err)
	}
	if got.Size != int64(len(body)) || got.ChecksumSHA256 == "" {
		t.Fatalf("bad metadata: %#v", got)
	}
	exists, err := s.Exists(ctx, got.Key)
	if err != nil || !exists {
		t.Fatalf("exists=%v err=%v", exists, err)
	}
	r, info, err := s.Open(ctx, got.Key)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(r)
	r.Close()
	if !bytes.Equal(raw, body) || info.Size != got.Size {
		t.Fatal("open mismatch")
	}
	if err := s.Copy(ctx, got.Key, "protected/test/b.bin"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SignedURL(ctx, got.Key, time.Minute); err == nil {
		t.Fatal("local signed URL must be unsupported")
	}
	if err := s.Delete(ctx, got.Key); err != nil {
		t.Fatal(err)
	}
	exists, _ = s.Exists(ctx, got.Key)
	if exists {
		t.Fatal("delete failed")
	}
}

func TestR2StoragePutSendsContentLength(t *testing.T) {
	body := []byte("r2-requires-a-known-content-length")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			raw, err := io.ReadAll(r.Body)
			if err != nil {
				t.Error(err)
			}
			if r.ContentLength != int64(len(body)) || !bytes.Equal(raw, body) {
				t.Errorf("content length/body mismatch: length=%d body=%q", r.ContentLength, raw)
			}
			w.WriteHeader(http.StatusOK)
		case http.MethodHead:
			w.Header().Set("Content-Length", "34")
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	cfg := aws.Config{
		Region:                     "auto",
		Credentials:                credentials.NewStaticCredentialsProvider("access", "secret", ""),
		RequestChecksumCalculation: aws.RequestChecksumCalculationWhenRequired,
		ResponseChecksumValidation: aws.ResponseChecksumValidationWhenRequired,
	}
	client := s3.NewFromConfig(cfg, func(options *s3.Options) {
		options.BaseEndpoint = aws.String(server.URL)
		options.UsePathStyle = true
	})
	storage := &R2Storage{client: client, bucket: "assets"}
	stored, err := storage.Put(context.Background(), PutObjectInput{Key: "uploads/test.webp", Body: bytes.NewReader(body), ContentType: "image/webp"})
	if err != nil {
		t.Fatal(err)
	}
	if stored.Size != int64(len(body)) || stored.ChecksumSHA256 == "" {
		t.Fatalf("bad R2 metadata: %#v", stored)
	}
}

func TestLocalStorageRejectsTraversal(t *testing.T) {
	s := &LocalStorage{Root: t.TempDir()}
	if _, err := s.Put(context.Background(), PutObjectInput{Key: "../escape", Body: bytes.NewReader(nil)}); err == nil {
		t.Fatal("expected traversal rejection")
	}
}

func TestContentTypeForLegacyWebPObject(t *testing.T) {
	for _, configured := range []string{"", "application/octet-stream", "application/octet-stream; charset=binary"} {
		if got := ContentTypeForObject("uploads/courses/course.webp", configured); got != "image/webp" {
			t.Fatalf("configured %q resolved to %q, want image/webp", configured, got)
		}
	}
	if got := ContentTypeForObject("uploads/courses/course.webp", "image/custom"); got != "image/custom" {
		t.Fatalf("valid configured MIME was replaced: %q", got)
	}
}
