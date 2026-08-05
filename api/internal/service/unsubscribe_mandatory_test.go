package service

import (
	"net/url"
	"testing"

	"github.com/mudgallabs/bodhveda/internal/email"
	"github.com/mudgallabs/bodhveda/internal/env"
	"github.com/mudgallabs/bodhveda/internal/model/dto"
)

// A mandatory (target, email) catalog entry must NOT get a List-Unsubscribe
// header.
//
// The regression: the send path used to build the unsubscribe URL
// unconditionally, while the endpoint behind it writes through
// PreferenceService.UpdateRecipientPreferenceTarget, whose refuseIfMandatory
// rejects a mandatory target with a 400. The result was a mail client showing an
// Unsubscribe chip that failed when clicked — on exactly the mail (password
// resets, security alerts) where a broken opt-out is least acceptable. A missing
// chip is honest; a dead one is not.
func TestBuildUnsubscribeURL_MandatorySuppressesHeader(t *testing.T) {
	withUnsubscribeEnv(t)

	s := &NotificationService{}
	target := dto.Target{Channel: "security", Topic: "none", Event: "password_reset"}

	if got := s.buildUnsubscribeURL(1, "user_1", target, true); got != "" {
		t.Errorf("mandatory target built an unsubscribe URL %q, want \"\" (no header)", got)
	}
}

// The guard must be narrow: a non-mandatory target still gets a working,
// verifiable URL. Without this, "suppress on mandatory" could regress into
// "suppress always" and silently strip every unsubscribe — a worse failure than
// the one being fixed, and an invisible one.
func TestBuildUnsubscribeURL_NonMandatoryStillSigned(t *testing.T) {
	withUnsubscribeEnv(t)

	s := &NotificationService{}
	target := dto.Target{Channel: "digest", Topic: "none", Event: "sent"}

	got := s.buildUnsubscribeURL(7, "user_1", target, false)
	if got == "" {
		t.Fatal("non-mandatory target built no unsubscribe URL, want one")
	}

	token := tokenFromURL(t, got)
	claims, err := email.ParseUnsubscribeToken(token, []byte(env.HashKey))
	if err != nil {
		t.Fatalf("token from built URL does not verify: %v", err)
	}

	if claims.ProjectID != 7 || claims.RecipientExtID != "user_1" {
		t.Errorf("claims = project %d/recipient %q, want 7/user_1", claims.ProjectID, claims.RecipientExtID)
	}
	if claims.Channel != target.Channel || claims.Topic != target.Topic || claims.Event != target.Event {
		t.Errorf("claims target = %s/%s/%s, want %s/%s/%s",
			claims.Channel, claims.Topic, claims.Event, target.Channel, target.Topic, target.Event)
	}
}

// With no public base URL configured there is nowhere for the header to point,
// so no header is attached regardless of the mandatory flag.
func TestBuildUnsubscribeURL_NoAPIURL(t *testing.T) {
	withUnsubscribeEnv(t)
	env.APIURL = ""

	s := &NotificationService{}
	target := dto.Target{Channel: "digest", Topic: "none", Event: "sent"}

	if got := s.buildUnsubscribeURL(1, "user_1", target, false); got != "" {
		t.Errorf("built %q with no APIURL, want \"\"", got)
	}
}

// withUnsubscribeEnv sets the process-wide env the token builder reads and
// restores it afterwards, so these tests can run in any order alongside others
// that touch env.
func withUnsubscribeEnv(t *testing.T) {
	t.Helper()

	apiURL, hashKey := env.APIURL, env.HashKey
	t.Cleanup(func() {
		env.APIURL, env.HashKey = apiURL, hashKey
	})

	env.APIURL = "https://api.bodhveda.test"
	env.HashKey = "test-hash-key-for-unsubscribe-tokens"
}

// tokenFromURL pulls the `t` query param out of a built unsubscribe URL.
func tokenFromURL(t *testing.T, raw string) string {
	t.Helper()

	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("built URL is not parseable: %v", err)
	}

	token := u.Query().Get("t")
	if token == "" {
		t.Fatalf("built URL %q carries no token", raw)
	}
	return token
}
