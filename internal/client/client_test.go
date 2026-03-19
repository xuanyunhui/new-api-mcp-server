package client

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClient_Do_RelayAuth(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "sk-relay-key", "sk-sys-key", 5*time.Second)

	resp, err := c.Do(context.Background(), SourceRelay, "POST", "/v1/chat/completions", nil, nil, []byte(`{}`))
	if err != nil {
		t.Fatalf("Do() error: %v", err)
	}
	defer resp.Body.Close()

	if gotAuth != "Bearer sk-relay-key" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer sk-relay-key")
	}
}

func TestClient_Do_APIAuth(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "sk-relay", "sk-sys-key", 5*time.Second)

	resp, err := c.Do(context.Background(), SourceAPI, "GET", "/api/channel/", nil, nil, nil)
	if err != nil {
		t.Fatalf("Do() error: %v", err)
	}
	defer resp.Body.Close()

	if gotAuth != "Bearer sk-sys-key" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer sk-sys-key")
	}
}

func TestClient_Do_QueryParams(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "sk-key", "", 5*time.Second)
	params := map[string]string{"page": "1", "limit": "10"}

	resp, err := c.Do(context.Background(), SourceRelay, "GET", "/api/items", params, nil, nil)
	if err != nil {
		t.Fatalf("Do() error: %v", err)
	}
	defer resp.Body.Close()

	if gotQuery == "" {
		t.Error("expected query params, got empty")
	}
}

func TestClient_Do_HeaderParams(t *testing.T) {
	var gotHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("x-api-key")
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "sk-key", "", 5*time.Second)
	headers := map[string]string{"x-api-key": "my-key"}

	resp, err := c.Do(context.Background(), SourceRelay, "GET", "/test", nil, headers, nil)
	if err != nil {
		t.Fatalf("Do() error: %v", err)
	}
	defer resp.Body.Close()

	if gotHeader != "my-key" {
		t.Errorf("x-api-key = %q, want %q", gotHeader, "my-key")
	}
}

func TestClient_Do_ReturnsBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"result":"hello"}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "sk-key", "", 5*time.Second)

	resp, err := c.Do(context.Background(), SourceRelay, "GET", "/test", nil, nil, nil)
	if err != nil {
		t.Fatalf("Do() error: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if string(body) != `{"result":"hello"}` {
		t.Errorf("body = %q, want %q", string(body), `{"result":"hello"}`)
	}
}
