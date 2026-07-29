package monitor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Discord embed colours (decimal RGB, as the API wants).
const (
	colourRed   = 0xE5484D // opened
	colourAmber = 0xF5A524 // ongoing
	colourGreen = 0x30A46C // resolved
)

// discordTimeout caps a single webhook POST. The monitor runs on the API
// process, so a hung Discord must not hold a tick open indefinitely.
const discordTimeout = 10 * time.Second

// Sink is where alerts go.
type Sink interface {
	Send(ctx context.Context, alert Alert) error
}

// DiscordSink posts alerts to a Discord incoming webhook.
type DiscordSink struct {
	url    string
	client *http.Client
}

// NewDiscordSink builds a sink for a Discord webhook URL. An EMPTY url yields a
// nil Sink, which the Monitor treats as log-only — see env.AlertDiscordWebhookURL.
//
// ⚠️ The return type is the Sink INTERFACE, not *DiscordSink, and that is
// load-bearing. Returning a typed nil pointer would produce a non-nil interface
// value once assigned to Config.Sink, so the Monitor's `sink == nil` log-only
// check would be false and every alert would panic on a nil receiver — turning
// "no Discord configured" into a crash on the first incident.
func NewDiscordSink(url string) Sink {
	if strings.TrimSpace(url) == "" {
		return nil
	}
	return &DiscordSink{
		url:    url,
		client: &http.Client{Timeout: discordTimeout},
	}
}

type discordPayload struct {
	Embeds []discordEmbed `json:"embeds"`
}

type discordEmbed struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Color       int    `json:"color"`
	Timestamp   string `json:"timestamp"`
	Footer      *struct {
		Text string `json:"text"`
	} `json:"footer,omitempty"`
}

// Send posts one alert. Errors are returned, never panicked — the Monitor logs
// them and carries on, because a broken alert channel must not take down the API
// process it runs inside.
func (s *DiscordSink) Send(ctx context.Context, alert Alert) error {
	body, err := json.Marshal(buildPayload(alert))
	if err != nil {
		return fmt.Errorf("marshal discord payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build discord request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("post to discord: %w", err)
	}
	defer resp.Body.Close()

	// Drain a bounded amount so the connection can be reused, and so a chatty
	// error response cannot be streamed at us indefinitely.
	snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("discord returned %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
	}

	return nil
}

func buildPayload(alert Alert) discordPayload {
	var description strings.Builder

	if alert.Kind == AlertResolved {
		// ⚠️ A resolve must NOT render like a live problem. Alert.Summary and
		// Alert.Fields describe the state while it was BROKEN, so echoing them
		// as-is produces a green "Resolved" card reading "No Asynq worker is
		// registered — check the worker container", which tells the reader to go
		// fix something that is already fixed. Lead with the recovery, and mark
		// the old summary as past tense.
		//
		// Fields are dropped entirely here: they are a snapshot of the broken
		// state, and the action-oriented ones ("hint: check the worker
		// container/process") are actively misleading after recovery.
		description.WriteString("Recovered after " + humanizeDuration(alert.ResolvedAfter) + ".")
		if alert.Summary != "" {
			description.WriteString("\n\nWas: " + alert.Summary)
		}

		return discordPayload{Embeds: []discordEmbed{finishEmbed(alert, description.String())}}
	}

	description.WriteString(alert.Summary)

	if lines := alert.FieldLines(); len(lines) > 0 {
		description.WriteString("\n\n```\n")
		description.WriteString(strings.Join(lines, "\n"))
		description.WriteString("\n```")
	}

	return discordPayload{Embeds: []discordEmbed{finishEmbed(alert, description.String())}}
}

// finishEmbed applies the parts common to every kind of alert.
func finishEmbed(alert Alert, description string) discordEmbed {
	embed := discordEmbed{
		Title:       alert.Title(),
		Description: description,
		Color:       colourFor(alert.Kind),
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
	}

	// On anything but a fresh open, say when this started — usually the first
	// thing you want to know when you read the message.
	if alert.Kind != AlertOpened && !alert.Since.IsZero() {
		embed.Footer = &struct {
			Text string `json:"text"`
		}{Text: "Started " + alert.Since.UTC().Format(time.RFC3339)}
	}

	return embed
}

// humanizeDuration renders a duration for a human reading an alert at a glance,
// not for a log: seconds below a minute, whole minutes below an hour, then
// hours and minutes. Go's default (`1h23m45.123456789s`) is unreadable here.
func humanizeDuration(d time.Duration) string {
	if d < time.Second {
		return "under a second"
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}

	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	if minutes == 0 {
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dh %dm", hours, minutes)
}

func colourFor(kind AlertKind) int {
	switch kind {
	case AlertResolved:
		return colourGreen
	case AlertOngoing:
		return colourAmber
	default:
		return colourRed
	}
}
