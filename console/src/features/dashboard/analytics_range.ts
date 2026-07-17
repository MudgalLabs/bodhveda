import { ProjectAnalyticsParams } from "@/features/dashboard/analytics_types";
import { formatDay, optionalDaySearch, optionalEnumSearch } from "@/lib/search";

// The analytics date range lives in the URL so a Home view is shareable and
// survives a reload — the same reason the notifications filters do (Phase 9.4).
//
// A PRESET (`7d`/`30d`/`90d`) is relative and re-resolves on load, so a shared
// "last 30 days" link keeps meaning the last 30 days. A CUSTOM range is two
// absolute calendar days (`from`/`to`) — stable across reload and timezone,
// exactly like the notifications filter. When `from`/`to` are present they win
// and the preset reads "Custom".
export const RANGE_PRESETS = ["7d", "30d", "90d"] as const;
export type RangePreset = (typeof RANGE_PRESETS)[number];

export const DEFAULT_RANGE_PRESET: RangePreset = "30d";

export const RANGE_PRESET_LABELS: Record<RangePreset, string> = {
    "7d": "Last 7 days",
    "30d": "Last 30 days",
    "90d": "Last 90 days",
};

const PRESET_DAYS: Record<RangePreset, number> = {
    "7d": 7,
    "30d": 30,
    "90d": 90,
};

export interface AnalyticsRange {
    preset: RangePreset;
    // Custom absolute range (calendar days, YYYY-MM-DD). Present ⇒ overrides the
    // preset.
    from?: string;
    to?: string;
}

/** Reads the Home route's URL search into a range selection. */
export function validateAnalyticsSearch(
    search: Record<string, unknown>
): AnalyticsRange {
    const from = optionalDaySearch(search.from);
    const to = optionalDaySearch(search.to);
    return {
        preset:
            optionalEnumSearch(search.preset, RANGE_PRESETS) ??
            DEFAULT_RANGE_PRESET,
        from,
        to,
    };
}

/** True when the selection is a custom absolute range (from and to both set). */
export function isCustomRange(range: AnalyticsRange): boolean {
    return !!range.from && !!range.to;
}

/**
 * A human label for the active range — for the "these numbers cover …" caption
 * that keeps a ranged dashboard from being mistaken for lifetime totals.
 */
export function rangeLabel(range: AnalyticsRange): string {
    if (isCustomRange(range)) return `${range.from} → ${range.to}`;
    return RANGE_PRESET_LABELS[range.preset];
}

function startOfLocalDay(day: string): Date {
    return new Date(`${day}T00:00:00`);
}

// End of a picked day, local. next-day-minus-1ms rather than 23:59:59.999 so a
// DST transition can't clip an hour (the Phase 9.4 convention).
function endOfLocalDay(day: string): Date {
    const next = startOfLocalDay(day);
    next.setDate(next.getDate() + 1);
    return new Date(next.getTime() - 1);
}

/**
 * Resolves the range to the absolute instants the API filters on.
 *
 * A preset is [start-of-day N-1 days ago, end-of-today], all in the viewer's
 * local timezone, so "last 30 days" is the operator's 30 days, not UTC's. Every
 * value is dropped rather than sent blank — the API decodes `created_from` into
 * a *time.Time and a blank param is a hard 400.
 */
export function rangeToParams(range: AnalyticsRange): ProjectAnalyticsParams {
    if (isCustomRange(range)) {
        return {
            created_from: startOfLocalDay(range.from!).toISOString(),
            created_to: endOfLocalDay(range.to!).toISOString(),
        };
    }

    const days = PRESET_DAYS[range.preset];
    const now = new Date();
    const today = formatDay(now);
    const start = new Date(now);
    start.setDate(start.getDate() - (days - 1));

    return {
        created_from: startOfLocalDay(formatDay(start)).toISOString(),
        created_to: endOfLocalDay(today).toISOString(),
    };
}

/** The range as Dates, for netra's range DatePicker (custom selection). */
export function rangeToDates(range: AnalyticsRange): Date[] {
    const dates: Date[] = [];
    if (range.from) dates.push(startOfLocalDay(range.from));
    if (range.to) dates.push(startOfLocalDay(range.to));
    return dates;
}

/**
 * The DatePicker's selection as `from`/`to` days. A half-picked range is an
 * open-ended `from` with no `to` — the Phase 9.4 rationale (netra's range picker
 * treats a two-date selection as complete, so closing on the first click makes a
 * range impossible to pick).
 */
export function datesToRange(dates: Date[]): { from?: string; to?: string } {
    return {
        from: dates[0] ? formatDay(dates[0]) : undefined,
        to: dates[1] ? formatDay(dates[1]) : undefined,
    };
}

/**
 * The inclusive calendar-day bounds of the active range, as `YYYY-MM-DD` in the
 * viewer's local timezone. Used to gap-fill the day axis: the server returns
 * only days with data, but a continuous axis needs every day in the window,
 * including empty ones at the edges.
 */
export function rangeDayBounds(range: AnalyticsRange): {
    start: string;
    end: string;
} {
    if (isCustomRange(range)) {
        return { start: range.from!, end: range.to! };
    }
    const days = PRESET_DAYS[range.preset];
    const now = new Date();
    const start = new Date(now);
    start.setDate(start.getDate() - (days - 1));
    return { start: formatDay(start), end: formatDay(now) };
}

/** Every `YYYY-MM-DD` day from `start` to `end` inclusive (local). */
export function enumerateDays(start: string, end: string): string[] {
    const days: string[] = [];
    const cur = new Date(`${start}T00:00:00`);
    const last = new Date(`${end}T00:00:00`);
    // Guard against a pathological range producing an unbounded loop.
    let guard = 0;
    while (cur <= last && guard < 1000) {
        days.push(formatDay(cur));
        cur.setDate(cur.getDate() + 1);
        guard++;
    }
    return days;
}

/** The viewer's IANA timezone, sent as X-Timezone so the API buckets per day in it. */
export function viewerTimezone(): string {
    try {
        return Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC";
    } catch {
        return "UTC";
    }
}
