package local

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sunnyhmz7010/CList/internal/storage"
)

type Backend struct {
	root string
}

func New(root string) *Backend {
	return &Backend{root: root}
}

func (b *Backend) Validate(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	info, err := os.Stat(b.root)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("local storage root is not a directory")
	}
	return nil
}

func (b *Backend) Capabilities() storage.Capabilities {
	return storage.Capabilities{Range: true, Head: true, Streaming: true}
}

func (b *Backend) HealthCheck(ctx context.Context) error {
	return b.Validate(ctx)
}

func (b *Backend) Put(ctx context.Context, reader io.Reader, metadata storage.ObjectMeta) (storage.Object, error) {
	finalPath, err := b.safePath(metadata.Key)
	if err != nil {
		return storage.Object{}, err
	}
	if err := os.MkdirAll(filepath.Dir(finalPath), 0o750); err != nil {
		return storage.Object{}, err
	}
	temporary, err := os.CreateTemp(filepath.Dir(finalPath), ".upload-*")
	if err != nil {
		return storage.Object{}, err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)

	hash := sha256.New()
	written, err := io.Copy(io.MultiWriter(temporary, hash), contextReader{ctx: ctx, reader: reader})
	if err != nil {
		_ = temporary.Close()
		return storage.Object{}, err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return storage.Object{}, err
	}
	if err := temporary.Close(); err != nil {
		return storage.Object{}, err
	}
	if err := os.Rename(temporaryPath, finalPath); err != nil {
		return storage.Object{}, err
	}
	return storage.Object{
		Key:    metadata.Key,
		Size:   written,
		SHA256: hex.EncodeToString(hash.Sum(nil)),
	}, nil
}

func (b *Backend) Open(ctx context.Context, key string, byteRange *storage.ByteRange) (storage.Reader, error) {
	select {
	case <-ctx.Done():
		return storage.Reader{}, ctx.Err()
	default:
	}
	path, err := b.safePath(key)
	if err != nil {
		return storage.Reader{}, err
	}
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return storage.Reader{}, storage.ErrObjectNotFound
	}
	if err != nil {
		return storage.Reader{}, err
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return storage.Reader{}, err
	}
	if byteRange == nil {
		return storage.Reader{ReadCloser: file, Size: info.Size()}, nil
	}
	if byteRange.Start < 0 || byteRange.End < byteRange.Start || byteRange.Start >= info.Size() || byteRange.End >= info.Size() {
		file.Close()
		return storage.Reader{}, storage.ErrInvalidRange
	}
	if _, err := file.Seek(byteRange.Start, io.SeekStart); err != nil {
		file.Close()
		return storage.Reader{}, err
	}
	length := byteRange.End - byteRange.Start + 1
	return storage.Reader{
		ReadCloser:   &limitedReadCloser{Reader: io.LimitReader(file, length), closer: file},
		Size:         length,
		ContentRange: formatContentRange(byteRange.Start, byteRange.End, info.Size()),
		Partial:      true,
	}, nil
}

func (b *Backend) Delete(ctx context.Context, key string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	path, err := b.safePath(key)
	if err != nil {
		return err
	}
	if err := os.Remove(path); errors.Is(err, os.ErrNotExist) {
		return nil
	} else {
		return err
	}
}

func (b *Backend) safePath(key string) (string, error) {
	if key == "" {
		return "", storage.ErrInvalidKey
	}
	cleanKey := filepath.Clean(filepath.FromSlash(key))
	if cleanKey == "." || filepath.IsAbs(cleanKey) || filepath.VolumeName(cleanKey) != "" {
		return "", storage.ErrPathEscape
	}
	root, err := filepath.Abs(b.root)
	if err != nil {
		return "", err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	candidate := filepath.Join(root, cleanKey)
	if !within(root, candidate) {
		return "", storage.ErrPathEscape
	}
	parent, err := resolveWithMissing(filepath.Dir(candidate))
	if err != nil {
		return "", err
	}
	candidate = filepath.Join(parent, filepath.Base(candidate))
	if !within(root, candidate) {
		return "", storage.ErrPathEscape
	}
	if info, err := os.Lstat(candidate); err == nil && info.Mode()&os.ModeSymlink != 0 {
		resolved, err := filepath.EvalSymlinks(candidate)
		if err != nil {
			return "", err
		}
		if !within(root, resolved) {
			return "", storage.ErrPathEscape
		}
		candidate = resolved
	}
	return candidate, nil
}

func resolveWithMissing(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	missing := make([]string, 0, 4)
	current := abs
	for {
		if _, err := os.Lstat(current); err == nil {
			resolved, err := filepath.EvalSymlinks(current)
			if err != nil {
				return "", err
			}
			for index := len(missing) - 1; index >= 0; index-- {
				resolved = filepath.Join(resolved, missing[index])
			}
			return resolved, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", os.ErrNotExist
		}
		missing = append(missing, filepath.Base(current))
		current = parent
	}
}

func within(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func formatContentRange(start, end, size int64) string {
	return "bytes " + strconv.FormatInt(start, 10) + "-" + strconv.FormatInt(end, 10) + "/" + strconv.FormatInt(size, 10)
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	select {
	case <-r.ctx.Done():
		return 0, r.ctx.Err()
	default:
		return r.reader.Read(p)
	}
}

type limitedReadCloser struct {
	io.Reader
	closer io.Closer
}

func (r *limitedReadCloser) Close() error {
	return r.closer.Close()
}
