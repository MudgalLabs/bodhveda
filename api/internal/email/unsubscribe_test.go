package email

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

// encodeSignedClaimsForTest builds a valid-signature token for the given claims
// verbatim (no TTL applied), so tests can craft an expired-but-signed token.
func encodeSignedClaimsForTest(t *testing.T, claims UnsubscribeClaims, key []byte) string {
	t.Helper()
	body, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(body) + "." + signUnsubscribePayload(body, key)
}

var testKey = []byte("test-hash-key-material-0123456789")

func TestUnsubscribeToken_RoundTrip(t *testing.T) {
	claims := UnsubscribeClaims{
		ProjectID:      42,
		RecipientExtID: "user-1",
		Channel:        "digest",
		Topic:          "none",
		Event:          "sent",
	}

	token, err := BuildUnsubscribeToken(claims, testKey)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	got, err := ParseUnsubscribeToken(token, testKey)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got.ProjectID != 42 || got.RecipientExtID != "user-1" ||
		got.Channel != "digest" || got.Topic != "none" || got.Event != "sent" {
		t.Fatalf("claims round-trip mismatch: %+v", got)
	}
	if got.ExpiresAt <= time.Now().Unix() {
		t.Errorf("expiry not in the future: %d", got.ExpiresAt)
	}
}

func TestUnsubscribeToken_TamperedSignature(t *testing.T) {
	token, err := BuildUnsubscribeToken(UnsubscribeClaims{ProjectID: 1, RecipientExtID: "r", Channel: "c", Topic: "t", Event: "e"}, testKey)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	// Flip the last character of the signature.
	tampered := token[:len(token)-1]
	if strings.HasSuffix(token, "a") {
		tampered += "b"
	} else {
		tampered += "a"
	}
	if _, err := ParseUnsubscribeToken(tampered, testKey); !errors.Is(err, ErrUnsubscribeTokenInvalid) {
		t.Fatalf("tampered signature: got %v, want ErrUnsubscribeTokenInvalid", err)
	}
}

func TestUnsubscribeToken_WrongKey(t *testing.T) {
	token, err := BuildUnsubscribeToken(UnsubscribeClaims{ProjectID: 1, RecipientExtID: "r", Channel: "c", Topic: "t", Event: "e"}, testKey)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if _, err := ParseUnsubscribeToken(token, []byte("a-different-hash-key-000000000000")); !errors.Is(err, ErrUnsubscribeTokenInvalid) {
		t.Fatalf("wrong key: got %v, want ErrUnsubscribeTokenInvalid", err)
	}
}

func TestUnsubscribeToken_Malformed(t *testing.T) {
	for _, tok := range []string{"", "no-dot", "onlyone.", ".onlysig", "!!!.@@@"} {
		if _, err := ParseUnsubscribeToken(tok, testKey); !errors.Is(err, ErrUnsubscribeTokenInvalid) {
			t.Errorf("malformed %q: got %v, want ErrUnsubscribeTokenInvalid", tok, err)
		}
	}
}

func TestUnsubscribeToken_Expired(t *testing.T) {
	// Hand-craft an expired-but-well-signed token (BuildUnsubscribeToken always
	// stamps a future expiry, so build the payload+sig directly).
	claims := UnsubscribeClaims{ProjectID: 1, RecipientExtID: "r", Channel: "c", Topic: "t", Event: "e", ExpiresAt: time.Now().Add(-time.Hour).Unix()}
	token := encodeSignedClaimsForTest(t, claims, testKey)
	if _, err := ParseUnsubscribeToken(token, testKey); !errors.Is(err, ErrUnsubscribeTokenExpired) {
		t.Fatalf("expired token: got %v, want ErrUnsubscribeTokenExpired", err)
	}
}

func TestUnsubscribeURL(t *testing.T) {
	got := UnsubscribeURL("https://api.bodhveda.com/", "abc.def")
	want := "https://api.bodhveda.com/unsubscribe/email?t=abc.def"
	if got != want {
		t.Errorf("UnsubscribeURL = %q, want %q", got, want)
	}
}
