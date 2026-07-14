package email

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mudgallabs/bodhveda/internal/model/enum"
)

// newTestResend points a ResendAdapter at a local test server instead of the
// real Resend API.
func newTestResend(apiKey, baseURL string) *ResendAdapter {
	a := NewResendAdapter(apiKey)
	a.baseURL = baseURL
	return a
}

func TestResendAdapter_Send_Success(t *testing.T) {
	var gotAuth, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"re_abc123"}`))
	}))
	defer srv.Close()

	adapter := newTestResend("re_test_key", srv.URL)

	res, err := adapter.Send(context.Background(), Message{
		FromName:    "Resurface",
		FromAddress: "hey@resurface.to",
		To:          "user@example.com",
		Subject:     "Your digest",
		HTML:        "<p>hi</p>",
		Text:        "hi",
	})
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}

	if res.Provider != enum.EmailProviderResend {
		t.Errorf("provider = %q, want %q", res.Provider, enum.EmailProviderResend)
	}
	if res.ProviderMessageID != "re_abc123" {
		t.Errorf("provider message id = %q, want re_abc123", res.ProviderMessageID)
	}
	if gotAuth != "Bearer re_test_key" {
		t.Errorf("Authorization = %q, want Bearer re_test_key", gotAuth)
	}

	// from-identity must be formatted "Name <address>".
	var sent resendSendRequest
	if err := json.Unmarshal([]byte(gotBody), &sent); err != nil {
		t.Fatalf("decode sent body: %v", err)
	}
	if sent.From != "Resurface <hey@resurface.to>" {
		t.Errorf("from = %q, want Resurface <hey@resurface.to>", sent.From)
	}
	if len(sent.To) != 1 || sent.To[0] != "user@example.com" {
		t.Errorf("to = %v, want [user@example.com]", sent.To)
	}
}

func TestResendAdapter_Send_FromAddressOnly(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		_, _ = w.Write([]byte(`{"id":"re_1"}`))
	}))
	defer srv.Close()

	adapter := newTestResend("k", srv.URL)
	_, err := adapter.Send(context.Background(), Message{
		FromAddress: "bare@example.com",
		To:          "user@example.com",
		Subject:     "s",
		Text:        "t",
	})
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}

	var sent resendSendRequest
	_ = json.Unmarshal([]byte(gotBody), &sent)
	if sent.From != "bare@example.com" {
		t.Errorf("from = %q, want bare@example.com (no name)", sent.From)
	}
}

func TestResendAdapter_Send_ProviderError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"name":"validation_error","message":"from domain not verified"}`))
	}))
	defer srv.Close()

	adapter := newTestResend("k", srv.URL)
	_, err := adapter.Send(context.Background(), Message{
		FromAddress: "x@example.com", To: "u@example.com", Subject: "s", Text: "t",
	})
	if err == nil {
		t.Fatal("expected error on non-2xx response, got nil")
	}
}
