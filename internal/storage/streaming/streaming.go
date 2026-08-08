package streaming

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/sunnyhmz7010/CList/internal/storage"
)

type Config struct {
	BaseURL    string
	BotToken   string
	ChannelID  string
	HTTPClient *http.Client
}

type Backend struct {
	baseURL   string
	token     string
	channelID string
	http      *http.Client
}

type apiResponse[T any] struct {
	OK          bool   `json:"ok"`
	Result      T      `json:"result"`
	Description string `json:"description"`
}

type messageResult struct {
	MessageID int64 `json:"message_id"`
	Chat      struct {
		ID int64 `json:"id"`
	} `json:"chat"`
	Document struct {
		FileID       string `json:"file_id"`
		FileUniqueID string `json:"file_unique_id"`
		FileSize     int64  `json:"file_size"`
	} `json:"document"`
}

func New(config Config) *Backend {
	client := config.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	return &Backend{
		baseURL:   strings.TrimRight(config.BaseURL, "/"),
		token:     config.BotToken,
		channelID: config.ChannelID,
		http:      client,
	}
}

func (b *Backend) Validate(ctx context.Context) error {
	_, err := b.call(ctx, "getMe", nil)
	return err
}

func (b *Backend) Capabilities() storage.Capabilities {
	return storage.Capabilities{Range: false, Head: false, Streaming: true}
}

func (b *Backend) HealthCheck(ctx context.Context) error {
	return b.Validate(ctx)
}

func (b *Backend) Put(ctx context.Context, reader io.Reader, meta storage.ObjectMeta) (storage.Object, error) {
	pipeReader, pipeWriter := io.Pipe()
	writer := multipart.NewWriter(pipeWriter)
	hash := sha256.New()
	go func() {
		if err := writer.WriteField("chat_id", b.channelID); err != nil {
			_ = pipeWriter.CloseWithError(err)
			return
		}
		part, err := writer.CreateFormFile("document", meta.FileName)
		if err != nil {
			_ = pipeWriter.CloseWithError(err)
			return
		}
		if _, err = io.Copy(part, io.TeeReader(reader, hash)); err != nil {
			_ = pipeWriter.CloseWithError(err)
			return
		}
		if err = writer.Close(); err != nil {
			_ = pipeWriter.CloseWithError(err)
			return
		}
		_ = pipeWriter.Close()
	}()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.methodURL("sendDocument"), pipeReader)
	if err != nil {
		return storage.Object{}, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	result, err := doJSON[messageResult](b.http, req)
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
	return storage.Object{Key: key, Size: ref.Size, SHA256: hex.EncodeToString(hash.Sum(nil)), Telegram: &ref}, nil
}

func (b *Backend) Open(ctx context.Context, key string, byteRange *storage.ByteRange) (storage.Reader, error) {
	if byteRange != nil {
		return storage.Reader{}, storage.ErrRangeUnsupported
	}
	ref, err := decodeRef(key)
	if err != nil {
		return storage.Reader{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.streamURL(ref.FileID), nil)
	if err != nil {
		return storage.Reader{}, err
	}
	response, err := b.http.Do(req)
	if err != nil {
		return storage.Reader{}, err
	}
	if response.StatusCode == http.StatusNotFound {
		response.Body.Close()
		return storage.Reader{}, storage.ErrObjectNotFound
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		response.Body.Close()
		return storage.Reader{}, fmt.Errorf("流式 API 返回 HTTP %d", response.StatusCode)
	}
	size := ref.Size
	if header := response.Header.Get("X-Telegram-File-Size"); header != "" {
		trustedSize, parseErr := strconv.ParseInt(header, 10, 64)
		if parseErr != nil || trustedSize < 0 {
			response.Body.Close()
			return storage.Reader{}, errors.New("流式 API 返回无效文件大小")
		}
		size = trustedSize
	} else if response.ContentLength >= 0 {
		size = response.ContentLength
	}
	return storage.Reader{ReadCloser: response.Body, Size: size}, nil
}

func (b *Backend) Delete(ctx context.Context, key string) error {
	ref, err := decodeRef(key)
	if err != nil {
		return err
	}
	_, err = b.call(ctx, "deleteMessage", url.Values{
		"chat_id":    {ref.ChatID},
		"message_id": {strconv.FormatInt(ref.MessageID, 10)},
	})
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "message to delete not found") {
		return nil
	}
	return err
}

func (b *Backend) methodURL(method string) string {
	return b.baseURL + "/bot" + url.PathEscape(b.token) + "/" + method
}

func (b *Backend) streamURL(fileID string) string {
	return b.baseURL + "/stream/file/bot" + url.PathEscape(b.token) + "/" + url.PathEscape(fileID)
}

func (b *Backend) call(ctx context.Context, method string, values url.Values) (json.RawMessage, error) {
	if values == nil {
		values = url.Values{}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.methodURL(method), strings.NewReader(values.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return doJSON[json.RawMessage](b.http, req)
}

func doJSON[T any](client *http.Client, req *http.Request) (T, error) {
	var zero T
	response, err := client.Do(req)
	if err != nil {
		return zero, err
	}
	defer response.Body.Close()
	var result apiResponse[T]
	if err := json.NewDecoder(io.LimitReader(response.Body, 2<<20)).Decode(&result); err != nil {
		return zero, err
	}
	if !result.OK {
		if result.Description == "" {
			return zero, fmt.Errorf("流式 API 返回 HTTP %d", response.StatusCode)
		}
		return zero, errors.New(result.Description)
	}
	return result.Result, nil
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
