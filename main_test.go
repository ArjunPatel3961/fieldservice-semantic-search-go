package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPostPreservesUpstreamStatus(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "upstream unavailable", http.StatusBadGateway)
	}))
	defer s.Close()

	err := (&Client{BaseURL: s.URL, HTTP: s.Client()}).post("/test", map[string]any{}, nil)
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("error %v is not an HTTPError", err)
	}
	if httpErr.StatusCode != http.StatusBadGateway {
		t.Fatalf("status=%d", httpErr.StatusCode)
	}
	if !strings.Contains(err.Error(), "upstream unavailable") {
		t.Fatalf("error %q omits response body", err)
	}
}

func TestPostReportsEnvelopeFailureWithoutError(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":false}`))
	}))
	defer s.Close()

	err := (&Client{BaseURL: s.URL, HTTP: s.Client()}).post("/test", map[string]any{}, nil)
	if err == nil || err.Error() == "" {
		t.Fatal("expected a non-empty error")
	}
}

func TestSearchRequiresQuery(t *testing.T) {
	cases := []struct{ name, body string }{{"empty", "{}"}, {"missing", ""}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Fatal("upstream should not be called") }))
			defer s.Close()
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/search", strings.NewReader(tc.body))
			searchHandler(&Client{BaseURL: s.URL, HTTP: s.Client()}, rr, req)
			if rr.Code != 400 {
				t.Fatalf("status=%d", rr.Code)
			}
		})
	}
}
func TestSearchEmbedsThenQueries(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/embeddings" {
			w.Write([]byte(`{"ok":true,"data":{"data":[{"embedding":[0.1,0.2]}]}}`))
			return
		}
		if r.URL.Path == "/v1/vector/query" {
			w.Write([]byte(`{"ok":true,"data":{"matches":[{"id":"wo-7"}]}}`))
			return
		}
		t.Fatalf("path %s", r.URL.Path)
	}))
	defer s.Close()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/search", strings.NewReader(`{"query":"pump leak"}`))
	searchHandler(&Client{BaseURL: s.URL, HTTP: s.Client()}, rr, req)
	if rr.Code != 200 {
		t.Fatalf("status=%d", rr.Code)
	}
}
