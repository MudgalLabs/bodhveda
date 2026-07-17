// Chart colors for the Home analytics (Phase 9.5).
//
// These are the console's OWN status tokens (src/index.css), so a chart reads
// the same as the rest of the console: delivered is the success green everywhere,
// bounced/failed the error red, muted the warning amber. They are a STATUS
// palette, not an arbitrary categorical one — the hues carry meaning, so they
// always ship with a legend + labels (never color alone).
//
// Validated (dataviz skill) against the dark card surface: adjacent-pair CVD
// separation ΔE 25.8 (protan) — comfortably above the 8 floor — normal-vision
// floor 30, chroma and 3:1 contrast all pass. The only failing check is the
// categorical lightness-band, which a status palette is expected to fail (green
// "good" is meant to look different from red "bad"); the always-present legend +
// value labels are the prescribed secondary encoding.
export const STATUS_COLORS = {
    delivered: "var(--color-success-foreground)",
    enqueued: "var(--color-azure-500)",
    muted: "var(--color-warning-foreground)",
    quota_exceeded: "#8b5cf6",
    failed: "var(--color-error-foreground)",
    // Email-specific outcomes reuse the same meaning-mapped hues.
    sent: "var(--color-azure-500)",
    pending: "var(--color-muted-foreground)",
    bounced: "var(--color-error-foreground)",
    complained: "#8b5cf6",
    no_contact: "var(--color-warning-foreground)",
} as const;

// A single sequential hue for magnitude-only charts (the target breakdown),
// where color encodes nothing — length does.
export const MAGNITUDE_COLOR = "var(--color-chart-primary)";

// The surface a chart sits on (a netra Card ≈ surface-2), for stack-segment gaps
// and mark rings.
export const CHART_SURFACE = "var(--color-surface-2)";
