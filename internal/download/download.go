package download

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

var client = &http.Client{Timeout: 10 * time.Minute}

func FileWithProgress(dest, source, label string, onProgress func(label string, done, total int64)) error {
	resp, err := client.Get(source)
	if err != nil {
		return fmt.Errorf("request unduhan gagal: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("server merespons HTTP %d", resp.StatusCode)
	}

	file, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("gagal membuat file unduhan: %w", err)
	}

	complete := false
	defer func() {
		if !complete {
			os.Remove(dest)
		}
	}()

	writer := io.Writer(file)
	if onProgress != nil {
		writer = io.MultiWriter(file, &progressWriter{
			label:      label,
			total:      resp.ContentLength,
			onProgress: onProgress,
		})
	}
	if _, err := io.Copy(writer, resp.Body); err != nil {
		file.Close()
		return fmt.Errorf("gagal menulis unduhan: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("gagal menutup file unduhan: %w", err)
	}

	complete = true
	return nil
}

type progressWriter struct {
	label      string
	total      int64
	done       int64
	onProgress func(label string, done, total int64)
}

func (w *progressWriter) Write(p []byte) (int, error) {
	w.done += int64(len(p))
	w.onProgress(w.label, w.done, w.total)
	return len(p), nil
}
