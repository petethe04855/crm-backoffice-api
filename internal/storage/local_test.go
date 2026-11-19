package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLocalRoundTrip(t *testing.T) {
	store, err := NewLocal(t.TempDir(), URLPrefix)
	require.NoError(t, err)

	url, err := store.Save(context.Background(), "abc-123", strings.NewReader("image-bytes"))
	require.NoError(t, err)
	require.Equal(t, "/api/v1/files/abc-123", url)

	rc, err := store.Open(context.Background(), "abc-123")
	require.NoError(t, err)
	defer rc.Close()
	got, err := io.ReadAll(rc)
	require.NoError(t, err)
	require.Equal(t, "image-bytes", string(got))

	// No .part file is left behind.
	entries, err := os.ReadDir(store.Dir())
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "abc-123", entries[0].Name())
}

func TestLocalRejectsTraversalNames(t *testing.T) {
	dir := t.TempDir()
	store, err := NewLocal(dir, URLPrefix)
	require.NoError(t, err)

	for _, name := range []string{"", "..", "../etc/passwd", `..\windows\win.ini`, "sub/dir", "a/../b"} {
		_, err := store.Save(context.Background(), name, bytes.NewReader([]byte("x")))
		require.Error(t, err, "Save should refuse %q", name)
		_, err = store.Open(context.Background(), name)
		require.Error(t, err, "Open should refuse %q", name)
	}

	// Nothing escaped the directory.
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Empty(t, entries)
}

func TestLocalOpenMissingFile(t *testing.T) {
	store, err := NewLocal(t.TempDir(), URLPrefix)
	require.NoError(t, err)

	_, err = store.Open(context.Background(), "missing")
	require.True(t, errors.Is(err, ErrNotFound))
}

func TestNewLocalCreatesTheDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "uploads")
	store, err := NewLocal(dir, "")
	require.NoError(t, err)
	info, err := os.Stat(dir)
	require.NoError(t, err)
	require.True(t, info.IsDir())
	require.Equal(t, dir, store.Dir())
}
