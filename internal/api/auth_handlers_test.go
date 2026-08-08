package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/sunnyhmz7010/CList/internal/auth"
	"github.com/sunnyhmz7010/CList/internal/db"
)

func TestAdminStatusReportsAuthenticatedSession(t *testing.T) {
	database, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "clist.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := db.Apply(context.Background(), database); err != nil {
		t.Fatal(err)
	}

	admin := auth.NewAdminService(database)
	if err := admin.Initialize(context.Background(), "admin", "correct horse battery"); err != nil {
		t.Fatal(err)
	}
	session, err := admin.Login(context.Background(), "admin", "correct horse battery")
	if err != nil {
		t.Fatal(err)
	}

	handler := NewAuthHandlers(admin, auth.NewAuthenticator(database))
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/status", nil)
	request.AddCookie(&http.Cookie{Name: auth.AdminCookieName, Value: session.Token})
	response := httptest.NewRecorder()

	handler.status(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var body map[string]bool
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if !body["initialized"] || !body["authenticated"] {
		t.Fatalf("got %#v", body)
	}
}
