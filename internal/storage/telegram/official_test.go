package telegram

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sunnyhmz7010/CList/internal/storage"
)

func TestOfficialPutSendsToConfiguredChannel(t *testing.T) {
	var gotChatID, gotName, gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/bottest-token/sendDocument" {
			t.Fatalf("请求路径 = %q", r.URL.Path)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatal(err)
		}
		gotChatID = r.FormValue("chat_id")
		file, header, err := r.FormFile("document")
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		gotName = header.Filename
		raw, err := io.ReadAll(file)
		if err != nil {
			t.Fatal(err)
		}
		gotBody = string(raw)
		writeTelegramJSON(t, w, map[string]any{
			"ok": true,
			"result": map[string]any{
				"message_id": 7,
				"chat":       map[string]any{"id": -100123},
				"document": map[string]any{
					"file_id":        "file-1",
					"file_unique_id": "unique-1",
					"file_size":      5,
				},
			},
		})
	}))
	defer server.Close()

	backend := New(Config{BaseURL: server.URL, BotToken: "test-token", ChannelID: "-100123", HTTPClient: server.Client()})
	object, err := backend.Put(context.Background(), strings.NewReader("hello"), storage.ObjectMeta{FileName: "示例.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if gotChatID != "-100123" || gotName != "示例.txt" || gotBody != "hello" {
		t.Fatalf("上传参数 = %q, %q, %q", gotChatID, gotName, gotBody)
	}
	if object.Telegram == nil || object.Telegram.MessageID != 7 || object.Size != 5 {
		t.Fatalf("对象元数据 = %#v", object)
	}
}

func TestOfficialOpenForwardsRange(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/bottest-token/getFile":
			writeTelegramJSON(t, w, map[string]any{
				"ok":     true,
				"result": map[string]any{"file_path": "documents/file.bin", "file_size": 10},
			})
		case "/file/bottest-token/documents/file.bin":
			if got := r.Header.Get("Range"); got != "bytes=2-5" {
				t.Fatalf("Range = %q", got)
			}
			w.Header().Set("Content-Range", "bytes 2-5/10")
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write([]byte("2345"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	backend := New(Config{BaseURL: server.URL, BotToken: "test-token", ChannelID: "-100123", HTTPClient: server.Client()})
	key, err := encodeRef(storage.TelegramRef{ChatID: "-100123", MessageID: 7, FileID: "file-1", Size: 10})
	if err != nil {
		t.Fatal(err)
	}
	reader, err := backend.Open(context.Background(), key, &storage.ByteRange{Start: 2, End: 5})
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	raw, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "2345" || !reader.Partial || reader.ContentRange != "bytes 2-5/10" {
		t.Fatalf("下载结果 = %q, partial=%v, contentRange=%q", raw, reader.Partial, reader.ContentRange)
	}
}

func TestOfficialDeleteTreatsMissingMessageAsSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeTelegramJSON(t, w, map[string]any{
			"ok":          false,
			"description": "Bad Request: message to delete not found",
		})
	}))
	defer server.Close()

	backend := New(Config{BaseURL: server.URL, BotToken: "test-token", ChannelID: "-100123", HTTPClient: server.Client()})
	key, err := encodeRef(storage.TelegramRef{ChatID: "-100123", MessageID: 7, FileID: "file-1"})
	if err != nil {
		t.Fatal(err)
	}
	if err := backend.Delete(context.Background(), key); err != nil {
		t.Fatal(err)
	}
}

func writeTelegramJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatal(err)
	}
}
