package preview

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"

	"github.com/sunnyhmz7010/CList/internal/auth"
	"github.com/sunnyhmz7010/CList/internal/files"
	"github.com/sunnyhmz7010/CList/internal/storage"
)

type Kind string

const (
	KindImage    Kind = "image"
	KindVideo    Kind = "video"
	KindAudio    Kind = "audio"
	KindPDF      Kind = "pdf"
	KindDOCX     Kind = "docx"
	KindText     Kind = "text"
	KindDownload Kind = "download"
)

type Preview struct {
	Kind Kind
	MIME string
	Body io.ReadCloser
	Size int64
}

var ErrTooLarge = errors.New("text preview is too large")

type Service struct {
	files    *files.FileService
	registry *storage.Registry
}

func NewService(fileService *files.FileService, registry *storage.Registry) *Service {
	return &Service{files: fileService, registry: registry}
}

func (s *Service) Open(ctx context.Context, publicID string, actor auth.Actor) (Preview, error) {
	file, err := s.files.Get(ctx, publicID, actor)
	if err != nil {
		return Preview{}, err
	}
	kind := kindFor(file.MIMEType, file.FileName)
	backend, err := s.registry.Resolve(file.StorageProfileID)
	if err != nil {
		return Preview{}, err
	}
	if kind == KindText {
		if file.Size > 2<<20 {
			return Preview{}, ErrTooLarge
		}
		reader, err := backend.Open(ctx, file.StorageKey, nil)
		if err != nil {
			return Preview{}, err
		}
		raw, err := io.ReadAll(io.LimitReader(reader, (2<<20)+1))
		reader.Close()
		if err != nil {
			return Preview{}, err
		}
		if len(raw) > 2<<20 {
			return Preview{}, ErrTooLarge
		}
		return Preview{Kind: KindText, MIME: "text/plain; charset=utf-8", Body: io.NopCloser(bytes.NewReader(raw)), Size: int64(len(raw))}, nil
	}
	reader, err := backend.Open(ctx, file.StorageKey, nil)
	if err != nil {
		return Preview{}, err
	}
	if kind == KindDOCX {
		raw, readErr := io.ReadAll(io.LimitReader(reader, maxDocxBytes+1))
		reader.Close()
		if readErr != nil {
			return Preview{}, readErr
		}
		text, err := ExtractDocxText(bytes.NewReader(raw))
		if err != nil {
			return Preview{}, err
		}
		return Preview{Kind: kind, MIME: "text/plain; charset=utf-8", Body: io.NopCloser(strings.NewReader(text)), Size: int64(len([]byte(text)))}, nil
	}
	return Preview{Kind: kind, MIME: file.MIMEType, Body: reader, Size: reader.Size}, nil
}

func kindFor(mime, name string) Kind {
	lowerName := strings.ToLower(name)
	switch {
	case strings.HasPrefix(mime, "image/"):
		return KindImage
	case strings.HasPrefix(mime, "video/"):
		return KindVideo
	case strings.HasPrefix(mime, "audio/"):
		return KindAudio
	case mime == "application/pdf":
		return KindPDF
	case strings.HasSuffix(lowerName, ".docx"):
		return KindDOCX
	case mime == "text/plain":
		return KindText
	default:
		return KindDownload
	}
}
