// A short, human day label ("Jul 3") for a chart axis / tooltip, from a
// `YYYY-MM-DD` day. Parsed as local midnight so the label matches the day the
// server bucketed in the viewer's timezone.
export function formatDayShort(day: string): string {
    const d = new Date(`${day}T00:00:00`);
    if (Number.isNaN(d.getTime())) return day;
    return d.toLocaleDateString(undefined, { month: "short", day: "numeric" });
}

/** A percentage with one decimal, e.g. 0.0123 → "1.2%". Blank denominator ⇒ "—". */
export function formatRate(numerator: number, denominator: number): string {
    if (denominator <= 0) return "—";
    return `${((numerator / denominator) * 100).toFixed(1)}%`;
}
