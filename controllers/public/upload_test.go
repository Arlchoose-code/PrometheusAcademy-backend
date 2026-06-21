package public

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"academyprometheus/backend/config"

	"github.com/gin-gonic/gin"
)

func TestServeUploadUsesImageMIMEAndSupportsHead(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	file := filepath.Join(root, "uploads", "courses", "example.webp")
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	body := []byte("webp-test-body")
	if err := os.WriteFile(file, body, 0o644); err != nil {
		t.Fatal(err)
	}

	controller := NewController(nil, config.Config{StoragePath: root, StorageProvider: "local"}, nil)
	router := gin.New()
	router.GET("/uploads/*filepath", controller.ServeUpload)
	router.HEAD("/uploads/*filepath", controller.ServeUpload)

	for _, method := range []string{http.MethodGet, http.MethodHead} {
		request := httptest.NewRequest(method, "/uploads/courses/example.webp", nil)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("%s status = %d", method, response.Code)
		}
		if got := response.Header().Get("Content-Type"); got != "image/webp" {
			t.Fatalf("%s content type = %q", method, got)
		}
		if method == http.MethodGet && response.Body.String() != string(body) {
			t.Fatalf("GET body = %q", response.Body.String())
		}
		if method == http.MethodHead && response.Body.Len() != 0 {
			t.Fatal("HEAD returned a response body")
		}
	}
}

func TestServeUploadDoesNotCacheMissingObjects(t *testing.T) {
	gin.SetMode(gin.TestMode)
	controller := NewController(nil, config.Config{StoragePath: t.TempDir(), StorageProvider: "local"}, nil)
	router := gin.New()
	router.GET("/uploads/*filepath", controller.ServeUpload)

	request := httptest.NewRequest(http.MethodGet, "/uploads/courses/missing.webp", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d", response.Code)
	}
	if got := response.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("cache control = %q", got)
	}
}
