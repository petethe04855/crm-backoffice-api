package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// DefaultDir is where images are stored when UPLOAD_DIR is not set.
const DefaultDir = "./uploads"

// URLPrefix is the public route that serves a stored file by name.
const URLPrefix = "/api/v1/files/"

// Local stores blobs as plain files in one directory on the API host.
type Local struct {
	dir       string
	urlPrefix string
}

// NewLocal prepares dir (creating it) and returns the store. urlPrefix lets
// tests use a fake route; production passes URLPrefix.
func NewLocal(dir, urlPrefix string) (*Local, error) {
	if strings.TrimSpace(dir) == "" {
		dir = DefaultDir
	}
	if urlPrefix == "" {
		urlPrefix = URLPrefix
	}
	if !strings.HasSuffix(urlPrefix, "/") {
		urlPrefix += "/"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create upload dir %q: %w", dir, err)
	}
	return &Local{dir: dir, urlPrefix: urlPrefix}, nil
}

// Dir is the directory the store writes to (used by tests and logs).
func (l *Local) Dir() string { return l.dir }

// Save writes r under name and returns its public URL. The write goes to a
// .part file first so a crashed upload never leaves a half image behind.
func (l *Local) Save(ctx context.Context, name string, r io.Reader) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := validateName(name); err != nil {
		return "", err
	}
	final := filepath.Join(l.dir, name)
	tmp := final + ".part"

	f, err := os.Create(tmp)
	if err != nil {
		return "", fmt.Errorf("create %s: %w", tmp, err)
	}
	if _, err := io.Copy(f, r); err != nil {
		f.Close()
		os.Remove(tmp)
		return "", fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return "", fmt.Errorf("close %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, final); err != nil {
		os.Remove(tmp)
		return "", fmt.Errorf("rename %s: %w", tmp, err)
	}
	return l.urlPrefix + name, nil
}

// Open returns the stored bytes for name.
func (l *Local) Open(ctx context.Context, name string) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := validateName(name); err != nil {
		return nil, err
	}
	f, err := os.Open(filepath.Join(l.dir, name))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrNotFound
	}
	return f, err
}

// validateName refuses anything that is not a plain file name, so a crafted id
// can never escape the upload directory.
func validateName(name string) error {
	if name == "" || name != filepath.Base(name) || strings.Contains(name, "..") ||
		strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("storage: invalid file name %q", name)
	}
	return nil
}
