package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
)

var errMessageNotFound = errors.New("telegram message not found")

type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

func NewClient(baseURL, token string, client *http.Client) *Client {
	if client == nil {
		client = http.DefaultClient
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		http:    client,
	}
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

type fileResult struct {
	FilePath string `json:"file_path"`
	FileSize int64  `json:"file_size"`
}

func (c *Client) SendMessage(ctx context.Context, chatID, text string, replyTo int64) error {
	values := url.Values{"chat_id": {chatID}, "text": {text}}
	if replyTo > 0 {
		values.Set("reply_to_message_id", fmt.Sprint(replyTo))
	}
	_, err := postForm[json.RawMessage](ctx, c, "sendMessage", values)
	return err
}

func (c *Client) SendDocument(ctx context.Context, chatID, fileName string, reader io.Reader) (messageResult, error) {
	pipeReader, pipeWriter := io.Pipe()
	writer := multipart.NewWriter(pipeWriter)
	go func() {
		if err := writer.WriteField("chat_id", chatID); err != nil {
			_ = pipeWriter.CloseWithError(err)
			return
		}
		part, err := writer.CreateFormFile("document", fileName)
		if err != nil {
			_ = pipeWriter.CloseWithError(err)
			return
		}
		if _, err = io.Copy(part, reader); err != nil {
			_ = pipeWriter.CloseWithError(err)
			return
		}
		if err = writer.Close(); err != nil {
			_ = pipeWriter.CloseWithError(err)
			return
		}
		_ = pipeWriter.Close()
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.methodURL("sendDocument"), pipeReader)
	if err != nil {
		return messageResult{}, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return doJSON[messageResult](c.http, req)
}

func (c *Client) GetFile(ctx context.Context, fileID string) (fileResult, error) {
	return postForm[fileResult](ctx, c, "getFile", url.Values{"file_id": {fileID}})
}

func (c *Client) DeleteMessage(ctx context.Context, chatID string, messageID int64) error {
	_, err := postForm[bool](ctx, c, "deleteMessage", url.Values{
		"chat_id":    {chatID},
		"message_id": {fmt.Sprint(messageID)},
	})
	if errors.Is(err, errMessageNotFound) {
		return nil
	}
	return err
}

func (c *Client) Health(ctx context.Context) error {
	_, err := postForm[json.RawMessage](ctx, c, "getMe", nil)
	return err
}

func (c *Client) methodURL(method string) string {
	return c.baseURL + "/bot" + url.PathEscape(c.token) + "/" + method
}

func (c *Client) fileURL(path string) string {
	parts := strings.Split(strings.TrimLeft(path, "/"), "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return c.baseURL + "/file/bot" + url.PathEscape(c.token) + "/" + strings.Join(parts, "/")
}

func postForm[T any](ctx context.Context, client *Client, method string, values url.Values) (T, error) {
	if values == nil {
		values = url.Values{}
	}
	body := strings.NewReader(values.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, client.methodURL(method), body)
	if err != nil {
		var zero T
		return zero, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return doJSON[T](client.http, req)
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
		if strings.Contains(strings.ToLower(result.Description), "message to delete not found") {
			return zero, errMessageNotFound
		}
		if result.Description == "" {
			return zero, fmt.Errorf("telegram API 返回 HTTP %d", response.StatusCode)
		}
		return zero, errors.New(result.Description)
	}
	return result.Result, nil
}
