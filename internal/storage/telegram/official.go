package telegram

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/sunnyhmz7010/CList/internal/storage"
)

type Config struct {
	BaseURL    string
	BotToken   string
	ChannelID  string
	HTTPClient *http.Client
}

type OfficialBackend struct {
	client    *Client
	channelID string
}

func New(config Config) *OfficialBackend {
	return &OfficialBackend{
		client:    NewClient(config.BaseURL, config.BotToken, config.HTTPClient),
		channelID: config.ChannelID,
	}
}

func (b *OfficialBackend) Validate(ctx context.Context) error {
	return b.client.Health(ctx)
}

func (b *OfficialBackend) Capabilities() storage.Capabilities {
	return storage.Capabilities{Range: true, Head: true, Streaming: true}
}

func (b *OfficialBackend) HealthCheck(ctx context.Context) error {
	return b.Validate(ctx)
}

func (b *OfficialBackend) Put(ctx context.Context, reader io.Reader, meta storage.ObjectMeta) (storage.Object, error) {
	hash := sha256.New()
	result, err := b.client.SendDocument(ctx, b.channelID, meta.FileName, io.TeeReader(reader, hash))
	if err != nil {
		return storage.Object{}, err
	}
	ref := storage.TelegramRef{
		ChatID:       strconv.FormatInt(result.Chat.ID, 10),
		MessageID:    result.MessageID,
		FileID:       result.Document.FileID,
		FileUniqueID: result.Document.FileUniqueID,
		Size:         result.Document.FileSize,
	}
	key, err := encodeRef(ref)
	if err != nil {
		return storage.Object{}, err
	}
	return storage.Object{
		Key:      key,
		Size:     ref.Size,
		SHA256:   hex.EncodeToString(hash.Sum(nil)),
		Telegram: &ref,
	}, nil
}

func (b *OfficialBackend) Open(ctx context.Context, key string, byteRange *storage.ByteRange) (storage.Reader, error) {
	ref, err := decodeRef(key)
	if err != nil {
		return storage.Reader{}, err
	}
	file, err := b.client.GetFile(ctx, ref.FileID)
	if err != nil {
		return storage.Reader{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.client.fileURL(file.FilePath), nil)
	if err != nil {
		return storage.Reader{}, err
	}
	if byteRange != nil {
		if byteRange.Start < 0 || byteRange.End < byteRange.Start {
			return storage.Reader{}, storage.ErrInvalidRange
		}
		req.Header.Set("Range", "bytes="+strconv.FormatInt(byteRange.Start, 10)+"-"+strconv.FormatInt(byteRange.End, 10))
	}

	response, err := b.client.http.Do(req)
	if err != nil {
		return storage.Reader{}, err
	}
	if response.StatusCode == http.StatusNotFound {
		response.Body.Close()
		return storage.Reader{}, storage.ErrObjectNotFound
	}
	if byteRange != nil && response.StatusCode != http.StatusPartialContent {
		response.Body.Close()
		return storage.Reader{}, storage.ErrRangeUnsupported
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		response.Body.Close()
		return storage.Reader{}, errors.New("Telegram 文件下载失败")
	}
	size := response.ContentLength
	if size < 0 {
		size = file.FileSize
	}
	return storage.Reader{
		ReadCloser:   response.Body,
		Size:         size,
		ContentRange: response.Header.Get("Content-Range"),
		Partial:      response.StatusCode == http.StatusPartialContent,
	}, nil
}

func (b *OfficialBackend) Delete(ctx context.Context, key string) error {
	ref, err := decodeRef(key)
	if err != nil {
		return err
	}
	return b.client.DeleteMessage(ctx, ref.ChatID, ref.MessageID)
}

func encodeRef(ref storage.TelegramRef) (string, error) {
	raw, err := json.Marshal(ref)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func decodeRef(key string) (storage.TelegramRef, error) {
	raw, err := base64.RawURLEncoding.DecodeString(key)
	if err != nil {
		return storage.TelegramRef{}, storage.ErrInvalidKey
	}
	var ref storage.TelegramRef
	if err = json.Unmarshal(raw, &ref); err != nil || ref.ChatID == "" || ref.MessageID == 0 || ref.FileID == "" {
		return storage.TelegramRef{}, storage.ErrInvalidKey
	}
	return ref, nil
}
