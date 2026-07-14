package email

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Unsubscribe (Phase 6).
//
// Outbound email carries an RFC 8058 List-Unsubscribe header pointing at a public
// Bodhveda endpoint. That endpoint takes no session/API key — it is hit from the
// mail client — so the URL carries a self-contained, signed token that identifies
// which recipient/target to unsubscribe from email. The token is:
//
//	base64url(payloadJSON) + "." + base64url(HMAC-SHA256(payloadJSON, HashKey))
//
// signed with BODHVEDA_API_HASH_KEY (the same key used to HMAC API-key tokens —
// NOT the cipher key). No DB row is needed: the endpoint re-derives the claims
// from the token and verifies the signature. The medium is always `email` (this is
// the email unsubscribe surface), so it is not carried in the claims.
var (
	// ErrUnsubscribeTokenInvalid means the token is malformed or its signature does
	// not verify (tampered). The endpoint maps it to 400/401.
	ErrUnsubscribeTokenInvalid = errors.New("unsubscribe token is invalid")
	// ErrUnsubscribeTokenExpired means the token's signature is valid but it is past
	// its expiry. The endpoint maps it to 401.
	ErrUnsubscribeTokenExpired = errors.New("unsubscribe token has expired")
)

// unsubscribeTokenTTL bounds how long an unsubscribe link stays valid after the
// email is sent. Generous, because a recipient may unsubscribe from an old email.
const unsubscribeTokenTTL = 180 * 24 * time.Hour

// UnsubscribeClaims are the self-contained claims a token carries. Short JSON keys
// keep the token compact.
type UnsubscribeClaims struct {
	ProjectID      int    `json:"p"`
	RecipientExtID string `json:"r"`
	Channel        string `json:"c"`
	Topic          string `json:"t"`
	Event          string `json:"e"`
	ExpiresAt      int64  `json:"exp"` // unix seconds
}

// BuildUnsubscribeToken signs the claims (with a TTL applied to ExpiresAt) into an
// opaque, URL-safe token.
func BuildUnsubscribeToken(claims UnsubscribeClaims, key []byte) (string, error) {
	claims.ExpiresAt = time.Now().Add(unsubscribeTokenTTL).Unix()

	body, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("marshal unsubscribe claims: %w", err)
	}

	payload := base64.RawURLEncoding.EncodeToString(body)
	sig := signUnsubscribePayload(body, key)
	return payload + "." + sig, nil
}

// ParseUnsubscribeToken verifies a token's signature and expiry and returns its
// claims. A malformed token or bad signature returns ErrUnsubscribeTokenInvalid; a
// well-signed but expired token returns ErrUnsubscribeTokenExpired.
func ParseUnsubscribeToken(token string, key []byte) (UnsubscribeClaims, error) {
	payload, sig, ok := strings.Cut(strings.TrimSpace(token), ".")
	if !ok || payload == "" || sig == "" {
		return UnsubscribeClaims{}, ErrUnsubscribeTokenInvalid
	}

	body, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return UnsubscribeClaims{}, ErrUnsubscribeTokenInvalid
	}

	expected := signUnsubscribePayload(body, key)
	if !hmac.Equal([]byte(sig), []byte(expected)) {
		return UnsubscribeClaims{}, ErrUnsubscribeTokenInvalid
	}

	var claims UnsubscribeClaims
	if err := json.Unmarshal(body, &claims); err != nil {
		return UnsubscribeClaims{}, ErrUnsubscribeTokenInvalid
	}

	if claims.ExpiresAt > 0 && time.Now().Unix() > claims.ExpiresAt {
		return UnsubscribeClaims{}, ErrUnsubscribeTokenExpired
	}

	return claims, nil
}

// UnsubscribeURL builds the public one-click unsubscribe URL for a token, given
// Bodhveda's own base URL (env.APIURL).
func UnsubscribeURL(baseURL, token string) string {
	base := strings.TrimRight(baseURL, "/")
	return fmt.Sprintf("%s/unsubscribe/email?t=%s", base, url.QueryEscape(token))
}

func signUnsubscribePayload(body, key []byte) string {
	mac := hmac.New(sha256.New, key)
	mac.Write(body)
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
