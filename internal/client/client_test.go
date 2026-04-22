package client

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDoSendsHeadersAndBody(t *testing.T) {
	var gotMethod, gotPath, gotAPIKey, gotContentType, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAPIKey = r.Header.Get("X-TYPESENSE-API-KEY")
		gotContentType = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "secret")
	body, status, err := c.Do(context.Background(), "POST", "/collections", []byte(`{"name":"x"}`))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if status != 201 {
		t.Errorf("status=%d", status)
	}
	if string(body) != `{"ok":true}` {
		t.Errorf("body=%s", body)
	}
	if gotMethod != "POST" || gotPath != "/collections" {
		t.Errorf("method/path: %s %s", gotMethod, gotPath)
	}
	if gotAPIKey != "secret" {
		t.Errorf("api key header: %q", gotAPIKey)
	}
	if gotContentType != "application/json" {
		t.Errorf("content-type: %q", gotContentType)
	}
	if gotBody != `{"name":"x"}` {
		t.Errorf("body: %q", gotBody)
	}
}

func TestDoReturnsNon2xxWithoutError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(422)
		_, _ = w.Write([]byte(`{"message":"bad"}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "k")
	body, status, err := c.Do(context.Background(), "GET", "/x", nil)
	if err != nil {
		t.Fatalf("unexpected error for 4xx: %v", err)
	}
	if status != 422 {
		t.Errorf("status=%d", status)
	}
	if !strings.Contains(string(body), "bad") {
		t.Errorf("body=%s", body)
	}
}

func TestDoTransportErrorReturnsError(t *testing.T) {
	c := New("http://127.0.0.1:1", "k") // port 1, nothing listens
	c.HTTP.Timeout = 200 * time.Millisecond
	_, _, err := c.Do(context.Background(), "GET", "/x", nil)
	if err == nil {
		t.Fatal("expected transport error, got nil")
	}
}

func TestDoRespectsContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	c := New(srv.URL, "k")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := c.Do(ctx, "GET", "/x", nil)
	if err == nil {
		t.Fatal("expected cancel error")
	}
}
