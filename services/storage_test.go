package services

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"
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

func TestLocalStorageRejectsTraversal(t *testing.T) {
	s := &LocalStorage{Root: t.TempDir()}
	if _, err := s.Put(context.Background(), PutObjectInput{Key: "../escape", Body: bytes.NewReader(nil)}); err == nil {
		t.Fatal("expected traversal rejection")
	}
}
