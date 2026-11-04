package download

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestFileWithProgress(t *testing.T) {
	const contents = "archive contents"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(contents))
	}))
	defer server.Close()

	dest := filepath.Join(t.TempDir(), "archive.zip")
	var progressCalls int
	if err := FileWithProgress(dest, server.URL, "archive", func(label string, done, total int64) {
		progressCalls++
		if label != "archive" || done != int64(len(contents)) || total != int64(len(contents)) {
			t.Errorf("unexpected progress: label=%q done=%d total=%d", label, done, total)
		}
	}); err != nil {
		t.Fatalf("FileWithProgress returned error: %v", err)
	}
	if progressCalls == 0 {
		t.Fatal("expected progress callback")
	}

	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read downloaded file: %v", err)
	}
	if string(data) != contents {
		t.Fatalf("downloaded content = %q, want %q", data, contents)
	}
}

func TestFileWithProgressRejectsHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	dest := filepath.Join(t.TempDir(), "archive.zip")
	if err := FileWithProgress(dest, server.URL, "archive", nil); err == nil {
		t.Fatal("expected HTTP status error")
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatalf("download destination should not exist; stat error = %v", err)
	}
}
