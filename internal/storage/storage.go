// Package storage keeps uploaded images outside the database, behind a small
// interface so a future S3/CDN backend can replace the local disk without
// touching handlers.
package storage

import (
	"context"
	"errors"
	"io"
)

// ErrNotFound means the stored blob no longer exists on the backend.
var ErrNotFound = errors.New("storage: file not found")

// Store writes and reads blobs by key.
type Store interface {
	// Save writes r under name and returns the public URL of the file.
	Save(ctx context.Context, name string, r io.Reader) (url string, err error)
	// Open returns the bytes stored under name (the key kept in the DB).
	Open(ctx context.Context, name string) (io.ReadCloser, error)
}
