package e2e

import (
	"net/http"
	"net/http/httptest"
)

type TestServer struct{ Server *httptest.Server }

func NewTestServer() *TestServer {
	handler := http.NewServeMux()
	handler.HandleFunc("/health/live", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	handler.HandleFunc("/health/ready", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	return &TestServer{Server: httptest.NewServer(handler)}
}

func (s *TestServer) Close() { s.Server.Close() }
