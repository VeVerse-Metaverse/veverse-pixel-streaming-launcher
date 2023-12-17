// Package http provides a set of functions to download files from the Internet and track download progress.
package http

import (
	"context"
	"fmt"
	"github.com/sirupsen/logrus"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// DownloadProgressTracker is a simple io.Writer that tracks the download progress using download state and callback function.
type DownloadProgressTracker struct {
	Current  uint64
	Total    uint64
	Progress func(current uint64, total uint64)
}

// NewDownloadProgressTracker creates a new DownloadProgressTracker.
func NewDownloadProgressTracker(total uint64, progress func(current uint64, total uint64)) *DownloadProgressTracker {
	return &DownloadProgressTracker{
		Total:    total,
		Progress: progress,
	}
}

// Write implements the io.Writer interface for the DownloadProgressTracker, triggering the progress callback when data is written.
func (c *DownloadProgressTracker) Write(p []byte) (int, error) {
	n := len(p)
	c.Current += uint64(n)
	if c.Progress != nil {
		c.Progress(c.Current, c.Total)
	}
	return n, nil
}

// DownloadFile downloads a file from the specified URL to the specified path.
func DownloadFile(ctx context.Context, path string, url string, counter *DownloadProgressTracker) (err error) {
	_, err1 := os.Stat(path)
	if err1 == nil {
		err2 := os.Remove(path)
		if err2 != nil {
			return fmt.Errorf("failed to remove file %s: %v", path, err2)
		}
	} else if !os.IsNotExist(err1) {
		return fmt.Errorf("failed to check if file exists: %v", err1)
	}

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to send a HTTP GET request: %s\n", err.Error())
	}
	defer func(body io.ReadCloser) {
		err := body.Close()
		if err != nil {
			logrus.Errorf("error closing http response body: %s\n", err)
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download file %s to %s: bad status: %s\n", url, path, resp.Status)
	}

	dir := filepath.Dir(path)
	err = os.MkdirAll(dir, 0750)
	if err != nil {
		return fmt.Errorf("failed to create a directory %s: %s\n", dir, err.Error())
	}

	out, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create a file downloaded %s to %s: %s\n", url, path, err.Error())
	}
	defer func(out *os.File) {
		err := out.Close()
		if err != nil {
			runtime.LogErrorf(ctx, "error closing file: %w\n", err)
		}
	}(out)

	// Write the body to file
	if counter != nil {
		counter.Total = uint64(resp.ContentLength)
		_, err = io.Copy(out, io.TeeReader(resp.Body, counter))
	} else {
		_, err = io.Copy(out, resp.Body)
	}
	if err != nil {
		return fmt.Errorf("failed to write a file downloaded %s to %s: %s\n", url, path, err.Error())
	}

	return nil
}
