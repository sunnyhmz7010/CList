package streaming

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sunnyhmz7010/CList/internal/storage"
)

func TestStreamingCapabilitiesRejectRange(t *testing.T) {
	backend := New(Config{BaseURL: "http://bot-api", BotToken: "t"})
	capabilities := backend.Capabilities()
	if capabilities.Range || capabilities.Head || !capabilities.Streaming {
		t.Fatalf("能力声明 = %+v", capabilities)
	}
	if _, err := backend.Open(context.Background(), "file-id", &storage.ByteRange{Start: 1, End: 2}); !errors.Is(err, storage.ErrRangeUnsupported) {
		t.Fatalf("错误 = %v", err)
	}
}

func TestStreamingOpenUsesEncodedFileIDAndTrustedSize(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/stream/file/bottest-token/file%2Fid%20with%20space" {
			t.Fatalf("路径 = %q", r.URL.EscapedPath())
		}
		w.Header().Set("X-Telegram-File-Size", "12")
		_, _ = w.Write([]byte("content"))
	}))
	defer server.Close()

	backend := New(Config{BaseURL: server.URL, BotToken: "test-token", HTTPClient: server.Client()})
	key, err := encodeRef(storage.TelegramRef{ChatID: "-1001", MessageID: 3, FileID: "file/id with space", Size: 9})
	if err != nil {
		t.Fatal(err)
	}
	reader, err := backend.Open(context.Background(), key, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	if reader.Size != 12 || reader.Partial {
		t.Fatalf("Reader = %+v", reader)
	}
}
