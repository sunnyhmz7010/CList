package webhook

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sunnyhmz7010/CList/internal/db"
)

func TestWebhookIsIdempotentAndRepliesStableLink(t *testing.T) {
	database, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "clist.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := db.Apply(context.Background(), database); err != nil {
		t.Fatal(err)
	}
	sender := &fakeSender{}
	service := NewService(database, &fakeResolver{profile: Profile{ID: "local-default", ChannelID: "-1001", Secret: "secret", Sender: sender}})
	handler := NewHandler(service)
	payload := `{"channel_post":{"message_id":42,"chat":{"id":-1001},"document":{"file_id":"file-1","file_unique_id":"unique-1","file_size":5,"file_name":"a.png","mime_type":"image/png"}}}`
	post := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/webhooks/telegram/public-secret", strings.NewReader(payload))
		req.Header.Set("X-Telegram-Bot-Api-Secret-Token", "secret")
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)
		return recorder
	}
	first, second := post(), post()
	if first.Code != http.StatusOK || second.Code != http.StatusOK {
		t.Fatalf("状态码 = %d, %d", first.Code, second.Code)
	}
	var count int
	if err := database.QueryRow("SELECT COUNT(*) FROM files").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 || len(sender.messages) != 1 || !strings.Contains(sender.messages[0], "/f/") {
		t.Fatalf("入库/回链 = %d, %#v", count, sender.messages)
	}
}

type fakeResolver struct{ profile Profile }

func (r *fakeResolver) Resolve(context.Context, string) (Profile, error) { return r.profile, nil }

type fakeSender struct{ messages []string }

func (s *fakeSender) SendMessage(_ context.Context, _, text string, _ int64) error {
	s.messages = append(s.messages, text)
	return nil
}
