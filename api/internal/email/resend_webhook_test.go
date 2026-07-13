package email

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"
)

// signSvix produces the headers Resend/Svix would send for a body signed with
// secret ("whsec_<base64>"), mirroring the verification the adapter performs.
func signSvix(t *testing.T, secret, msgID string, ts int64, body []byte) http.Header {
	t.Helper()
	key, err := base64.StdEncoding.DecodeString(secret[len(svixSecretPrefix):])
	if err != nil {
		t.Fatalf("decode secret: %v", err)
	}
	tsStr := strconv.FormatInt(ts, 10)
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(msgID + "." + tsStr + "."))
	mac.Write(body)
	sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	h := http.Header{}
	h.Set("svix-id", msgID)
	h.Set("svix-timestamp", tsStr)
	h.Set("svix-signature", "v1,"+sig)
	return h
}

func testSecret(t *testing.T) string {
	t.Helper()
	return svixSecretPrefix + base64.StdEncoding.EncodeToString([]byte("super-secret-key-material"))
}

func TestVerifyWebhookSignature_Valid(t *testing.T) {
	a := NewResendAdapter("")
	secret := testSecret(t)
	body := []byte(`{"type":"email.delivered","data":{"email_id":"abc"}}`)
	h := signSvix(t, secret, "msg_1", time.Now().Unix(), body)

	if err := a.VerifyWebhookSignature(secret, h, body); err != nil {
		t.Fatalf("expected valid signature, got %v", err)
	}
}

func TestVerifyWebhookSignature_TamperedBody(t *testing.T) {
	a := NewResendAdapter("")
	secret := testSecret(t)
	body := []byte(`{"type":"email.delivered","data":{"email_id":"abc"}}`)
	h := signSvix(t, secret, "msg_1", time.Now().Unix(), body)

	// Signature was computed over the original body; a different body must fail.
	if err := a.VerifyWebhookSignature(secret, h, []byte(`{"type":"email.bounced"}`)); err != ErrWebhookSignatureInvalid {
		t.Fatalf("expected ErrWebhookSignatureInvalid, got %v", err)
	}
}

func TestVerifyWebhookSignature_WrongSecret(t *testing.T) {
	a := NewResendAdapter("")
	body := []byte(`{"type":"email.delivered"}`)
	h := signSvix(t, testSecret(t), "msg_1", time.Now().Unix(), body)

	other := svixSecretPrefix + base64.StdEncoding.EncodeToString([]byte("a-different-key"))
	if err := a.VerifyWebhookSignature(other, h, body); err != ErrWebhookSignatureInvalid {
		t.Fatalf("expected ErrWebhookSignatureInvalid, got %v", err)
	}
}

func TestVerifyWebhookSignature_StaleTimestamp(t *testing.T) {
	a := NewResendAdapter("")
	secret := testSecret(t)
	body := []byte(`{"type":"email.delivered"}`)
	// An hour in the past is well beyond the tolerance window (replay).
	h := signSvix(t, secret, "msg_1", time.Now().Add(-time.Hour).Unix(), body)

	if err := a.VerifyWebhookSignature(secret, h, body); err != ErrWebhookSignatureInvalid {
		t.Fatalf("expected ErrWebhookSignatureInvalid for stale timestamp, got %v", err)
	}
}

func TestVerifyWebhookSignature_MissingHeaders(t *testing.T) {
	a := NewResendAdapter("")
	if err := a.VerifyWebhookSignature(testSecret(t), http.Header{}, []byte(`{}`)); err != ErrWebhookSignatureInvalid {
		t.Fatalf("expected ErrWebhookSignatureInvalid for missing headers, got %v", err)
	}
}

func TestNormalizeWebhookEvent_Mapping(t *testing.T) {
	a := NewResendAdapter("")
	cases := []struct {
		eventType string
		want      WebhookEventKind
	}{
		{"email.sent", WebhookEventSent},
		{"email.delivered", WebhookEventDelivered},
		{"email.bounced", WebhookEventBounced},
		{"email.complained", WebhookEventComplained},
		{"email.opened", WebhookEventOpened},
		{"email.clicked", WebhookEventClicked},
		{"email.delivery_delayed", WebhookEventUnknown},
	}
	for _, c := range cases {
		body := []byte(fmt.Sprintf(`{"type":%q,"created_at":"2026-07-13T10:00:00Z","data":{"email_id":"msg_%s"}}`, c.eventType, c.eventType))
		ev, err := a.NormalizeWebhookEvent(http.Header{}, body)
		if err != nil {
			t.Fatalf("%s: unexpected error %v", c.eventType, err)
		}
		if ev.Kind != c.want {
			t.Errorf("%s: kind = %q, want %q", c.eventType, ev.Kind, c.want)
		}
		if c.want != WebhookEventUnknown {
			if ev.ProviderMessageID != "msg_"+c.eventType {
				t.Errorf("%s: provider message id = %q", c.eventType, ev.ProviderMessageID)
			}
			if ev.At.IsZero() {
				t.Errorf("%s: expected parsed timestamp", c.eventType)
			}
		}
	}
}
