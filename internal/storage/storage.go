package storage

import (
	"context"
	"errors"
	"io"
)

var (
	ErrPathEscape       = errors.New("storage path escapes root")
	ErrInvalidKey       = errors.New("invalid storage key")
	ErrRangeUnsupported = errors.New("range is unsupported")
	ErrInvalidRange     = errors.New("invalid byte range")
	ErrBackendNotFound  = errors.New("storage backend not found")
	ErrObjectNotFound   = errors.New("storage object not found")
)

type ByteRange struct {
	Start int64
	End   int64
}

type Capabilities struct {
	Range     bool
	Head      bool
	Streaming bool
}

type ObjectMeta struct {
	Key      string
	FileName string
	MIMEType string
	Size     int64
}

type TelegramRef struct {
	ChatID       string
	MessageID    int64
	FileID       string
	FileUniqueID string
	Size         int64
}

type Object struct {
	Key      string
	Size     int64
	SHA256   string
	Telegram *TelegramRef
}

type Reader struct {
	io.ReadCloser
	Size         int64
	ContentRange string
	Partial      bool
}

type Backend interface {
	Validate(ctx context.Context) error
	Capabilities() Capabilities
	HealthCheck(ctx context.Context) error
	Put(ctx context.Context, reader io.Reader, metadata ObjectMeta) (Object, error)
	Open(ctx context.Context, key string, byteRange *ByteRange) (Reader, error)
	Delete(ctx context.Context, key string) error
}
