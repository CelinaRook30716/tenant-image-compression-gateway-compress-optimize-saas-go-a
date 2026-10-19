package infrai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCompressRetriesRateLimitWithSameIdempotencyKey(t *testing.T) {
	var calls int
	var keys []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		keys = append(keys, r.Header.Get("Idempotency-Key"))
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("authorization header missing")
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("content type = %q", r.Header.Get("Content-Type"))
		}
		var payload map[string]map[string]string
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if len(payload) != 1 || len(payload["image"]) != 1 || payload["image"]["base64"] == "" {
			t.Fatalf("unexpected image payload: %v", payload)
		}
		content, err := base64.StdEncoding.DecodeString(payload["image"]["base64"])
		if err != nil {
			t.Fatal(err)
		}
		if string(content) != "pixels" {
			t.Errorf("image = %q", content)
		}
		w.Header().Set("Content-Type", "application/json")
		if calls == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			io.WriteString(w, `{"ok":false,"data":null,"error":{"code":"RATE_LIMITED","message":"retry later"},"metadata":{}}`)
			return
		}
		io.WriteString(w, `{"ok":true,"data":{"id":"img_123"},"error":null,"metadata":{"vendor":"auto"}}`)
	}))
	defer server.Close()

	client := Compressor{
		APIKey: "test-key",
		Client: server.Client(),
		URL:    server.URL,
		Sleep:  func(context.Context, time.Duration) error { return nil },
	}
	data, err := client.Compress(context.Background(), "hero.png", []byte("pixels"), "tenant-7:upload-9")
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	if result.ID != "img_123" || calls != 2 {
		t.Fatalf("result = %+v, calls = %d", result, calls)
	}
	if strings.Join(keys, ",") != "tenant-7:upload-9,tenant-7:upload-9" {
		t.Fatalf("idempotency keys = %v", keys)
	}
}

func TestCompressReturnsBusinessEnvelopeBeforeStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `{"ok":false,"data":null,"error":{"code":"INVALID_ARGUMENT","message":"image is required"},"metadata":{}}`)
	}))
	defer server.Close()

	client := Compressor{APIKey: "test-key", Client: server.Client(), URL: server.URL}
	_, err := client.Compress(context.Background(), "hero.png", []byte("pixels"), "upload-9")
	apiErr, ok := err.(*APIError)
	if !ok || apiErr.Code != "INVALID_ARGUMENT" || apiErr.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("error = %#v", err)
	}
}
