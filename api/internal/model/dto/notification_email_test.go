package dto

import (
	"strings"
	"testing"

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
	base := func() SendNotificationPayload {
		return SendNotificationPayload{
			ProjectID:      1,
			RecipientExtID: strptr("user_1"),
			Target:         &Target{Channel: "digest", Topic: "none", Event: "sent"},
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
