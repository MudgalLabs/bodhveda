package dto

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/tantra/service"
)

func strptr(s string) *string { return &s }

// hasErrorFor reports whether err is an InputValidationErrors carrying an entry
// whose property path matches. (InputValidationErrors.Error() returns "" by
// design, so callers must inspect the typed value.)
func hasErrorFor(err error, propertyPath string) bool {
	errs, ok := err.(service.InputValidationErrors)
	if !ok {
		return false
	}
	for _, e := range errs {
		if e.PropertyPath == propertyPath {
			return true
		}
	}
	return false
}

func TestSendNotificationPayload_Validate_EmailBlock(t *testing.T) {
	// A valid direct send carries an in-app content block. It is set explicitly
	// because `payload` is no longer required by the type — a send with neither
	// `payload` nor `email` names no medium and is rejected (see
	// TestSendNotificationPayload_Validate_ContentBlocks).
	base := func() SendNotificationPayload {
		return SendNotificationPayload{
			ProjectID:      1,
			RecipientExtID: strptr("user_1"),
			Target:         &Target{Channel: "digest", Topic: "none", Event: "sent"},
			Payload:        json.RawMessage(`{"title":"hi"}`),
		}
	}

	t.Run("valid email block passes", func(t *testing.T) {
		p := base()
		p.Email = &EmailContent{Subject: "Hi", HTML: "<p>x</p>"}
		if err := p.Validate(); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("email on broadcast is rejected", func(t *testing.T) {
		p := base()
		p.RecipientExtID = nil // broadcast
		p.Email = &EmailContent{Subject: "Hi", Text: "x"}
		err := p.Validate()
		if err == nil || !hasErrorFor(err, "email") {
			t.Fatalf("expected broadcast rejection on 'email', got %v", err)
		}
	})

	t.Run("missing subject is rejected", func(t *testing.T) {
		p := base()
		p.Email = &EmailContent{HTML: "<p>x</p>"}
		if err := p.Validate(); err == nil {
			t.Fatal("expected error for missing subject")
		}
	})

	t.Run("missing html and text is rejected", func(t *testing.T) {
		p := base()
		p.Email = &EmailContent{Subject: "Hi"}
		if err := p.Validate(); err == nil {
			t.Fatal("expected error for missing content")
		}
	})

	t.Run("no email block is fine", func(t *testing.T) {
		p := base()
		if err := p.Validate(); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("broadcast with no target reports target error (no panic)", func(t *testing.T) {
		p := SendNotificationPayload{ProjectID: 1} // broadcast, Target nil
		err := p.Validate()
		if err == nil || !hasErrorFor(err, "target") {
			t.Fatalf("expected a 'target' validation error, got %v", err)
		}
	})
}

// TestSendNotificationPayload_Validate_ContentBlocks covers the rule that makes
// an email-only send expressible: a medium fires iff its content block is
// present, so `payload` is optional on a direct send — but a send carrying
// NEITHER block names no medium and is rejected rather than silently doing
// nothing.
func TestSendNotificationPayload_Validate_ContentBlocks(t *testing.T) {
	direct := func() SendNotificationPayload {
		return SendNotificationPayload{
			ProjectID:      1,
			RecipientExtID: strptr("user_1"),
			Target:         &Target{Channel: "conversation", Topic: "thread_7", Event: "reply"},
		}
	}

	t.Run("email-only direct send is accepted", func(t *testing.T) {
		p := direct()
		p.Email = &EmailContent{Subject: "3 new messages", HTML: "<p>x</p>"}
		if err := p.Validate(); err != nil {
			t.Fatalf("expected an email-only direct send to be valid, got %v", err)
		}
		if p.HasPayload() {
			t.Fatal("expected HasPayload to be false with no payload block")
		}
	})

	t.Run("neither block is rejected", func(t *testing.T) {
		p := direct()
		err := p.Validate()
		if err == nil || !hasErrorFor(err, "payload") {
			t.Fatalf("expected a 'payload' validation error for a send with no content block, got %v", err)
		}
	})

	t.Run("explicit JSON null payload counts as absent", func(t *testing.T) {
		// An omitted field and an explicit `"payload": null` mean the same thing
		// to a caller, so they must validate the same way.
		p := direct()
		p.Payload = json.RawMessage(`null`)
		err := p.Validate()
		if err == nil || !hasErrorFor(err, "payload") {
			t.Fatalf("expected `null` payload to be treated as absent, got %v", err)
		}
	})

	t.Run("payload-less broadcast is rejected", func(t *testing.T) {
		// Broadcasts are in-app only, so the at-least-one-block rule is not enough
		// — an `email` block cannot stand in for the missing payload.
		p := direct()
		p.RecipientExtID = nil
		p.Email = &EmailContent{Subject: "Hi", Text: "x"}
		err := p.Validate()
		if err == nil {
			t.Fatal("expected a payload-less broadcast to be rejected")
		}
		if !hasErrorFor(err, "payload") {
			t.Errorf("expected a 'payload' error explaining broadcasts require one, got %v", err)
		}
		if !hasErrorFor(err, "email") {
			t.Errorf("expected the 'email' error too — a caller needs both halves, got %v", err)
		}
	})

	t.Run("in-app-only direct send is still accepted", func(t *testing.T) {
		p := direct()
		p.Payload = json.RawMessage(`{"title":"hi"}`)
		if err := p.Validate(); err != nil {
			t.Fatalf("expected an in-app-only send to remain valid, got %v", err)
		}
	})
}

// TestIsJSONContent pins the nil-vs-`null` distinction that the whole email-only
// path depends on. A nil json.RawMessage MARSHALS to `null` and UNMARSHALS back
// to the 4-byte slice `null`, not to nil — so a payload-less notification
// arrives at the worker through its Asynq task payload as `null`. A len() check
// would report that as in-app content and write the very inbox row an email-only
// send exists to avoid.
func TestIsJSONContent(t *testing.T) {
	cases := []struct {
		name string
		raw  json.RawMessage
		want bool
	}{
		{"nil slice", nil, false},
		{"empty slice", json.RawMessage{}, false},
		{"literal null", json.RawMessage(`null`), false},
		{"null with whitespace", json.RawMessage("  null\n"), false},
		{"object", json.RawMessage(`{"a":1}`), true},
		{"empty object", json.RawMessage(`{}`), true},
		{"string", json.RawMessage(`"hi"`), true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsJSONContent(tc.raw); got != tc.want {
				t.Fatalf("IsJSONContent(%q) = %v, want %v", string(tc.raw), got, tc.want)
			}
		})
	}
}

// TestNotificationPayloadRoundTripsThroughTaskPayload proves the trap above is
// real rather than theoretical: it marshals and unmarshals a payload-less
// notification exactly as the Asynq queue does, and asserts the worker's
// "was in-app requested?" test still answers no on the other side.
func TestNotificationPayloadRoundTripsThroughTaskPayload(t *testing.T) {
	var absent json.RawMessage // an email-only send stores SQL NULL -> nil here

	encoded, err := json.Marshal(NotificationDeliveryTaskPayload{
		UserID:       1,
		Notification: &entity.Notification{ID: 42, Payload: absent},
		Email:        &EmailContent{Subject: "3 new messages", Text: "x"},
	})
	if err != nil {
		t.Fatalf("marshal task payload: %v", err)
	}

	var decoded NotificationDeliveryTaskPayload
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal task payload: %v", err)
	}

	// The round trip really does turn nil into the 4-byte slice `null` — if this
	// ever stops being true the guard below is still correct, but the comment
	// explaining why it exists would be stale.
	if len(decoded.Notification.Payload) == 0 {
		t.Log("note: nil payload survived the round trip as empty, not `null`")
	}

	if IsJSONContent(decoded.Notification.Payload) {
		t.Fatalf("payload %q read as in-app content after a queue round trip; the worker would write an inbox row for an email-only send", string(decoded.Notification.Payload))
	}
}

func TestEmailContent_ResolvedText(t *testing.T) {
	t.Run("uses text when present", func(t *testing.T) {
		e := &EmailContent{Text: "explicit", HTML: "<p>ignored</p>"}
		if got := e.ResolvedText(); got != "explicit" {
			t.Errorf("got %q, want explicit", got)
		}
	})

	t.Run("derives from html when text omitted", func(t *testing.T) {
		e := &EmailContent{HTML: "<h1>Hello</h1><p>World &amp; more</p>"}
		got := e.ResolvedText()
		if !strings.Contains(got, "Hello") || !strings.Contains(got, "World") {
			t.Errorf("derived text %q missing expected words", got)
		}
		if strings.Contains(got, "<") {
			t.Errorf("derived text %q still contains tags", got)
		}
		// Entities are decoded, not passed through raw.
		if !strings.Contains(got, "World & more") {
			t.Errorf("derived text %q did not decode &amp;", got)
		}
	})

	t.Run("drops style/script/head content", func(t *testing.T) {
		e := &EmailContent{HTML: `<head><style>.x{color:red}</style></head><body><script>alert(1)</script><p>Visible</p></body>`}
		got := e.ResolvedText()
		if !strings.Contains(got, "Visible") {
			t.Errorf("derived text %q missing body text", got)
		}
		for _, leak := range []string{"color:red", ".x{", "alert(1)"} {
			if strings.Contains(got, leak) {
				t.Errorf("derived text %q leaked non-body content %q", got, leak)
			}
		}
	})
}
