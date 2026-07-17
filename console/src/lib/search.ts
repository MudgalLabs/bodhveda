/**
 * Builds a route's `validateSearch` for a param that selects one of a fixed set
 * of views — a tab, a kind toggle — so the choice survives a refresh and travels
 * with a shared link.
 *
 * It always resolves to a concrete value rather than omitting an unrecognized
 * one: an omitted key is NOT stripped from the search the component reads, so
 * `?tab=bogus` would reach the control as-is, match none of its options, and
 * leave the page with nothing selected and an empty panel below it.
 */
export function validateViewSearch<K extends string, V extends string>(
    key: K,
    allowed: readonly V[],
    fallback: V
): (search: Record<string, unknown>) => Record<K, V> {
    return (search) => {
        const value = search[key];
        return {
            [key]: allowed.includes(value as V) ? (value as V) : fallback,
        } as Record<K, V>;
    };
}

/**
 * Reads an OPTIONAL search param constrained to a fixed set.
 *
 * Unlike validateViewSearch this drops an unrecognized value instead of falling
 * back, because the two serve opposite jobs: a view param must always name a
 * concrete view, whereas an absent filter is a perfectly good filter — it means
 * "don't narrow". Dropping also keeps a hand-edited `?status=bogus` from
 * silently behaving like a filter nothing can match.
 */
export function optionalEnumSearch<V extends string>(
    value: unknown,
    allowed: readonly V[]
): V | undefined {
    return allowed.includes(value as V) ? (value as V) : undefined;
}

/**
 * Reads an optional free-text search param, treating blank as absent.
 *
 * Numbers and booleans are coerced back to text rather than rejected, because
 * TanStack Router PARSES the search string: `?recipient_search=12` arrives as
 * the number 12, and `?channel=true` as the boolean true. A `typeof === "string"`
 * guard therefore silently drops any filter whose value merely looks like
 * another type — and recipient external ids are customer-chosen strings that are
 * very often all digits (`123`, `42`). These are text filters over text columns;
 * what the router inferred about the shape of the value is not information.
 */
export function optionalStringSearch(value: unknown): string | undefined {
    if (typeof value !== "string" && typeof value !== "number" && typeof value !== "boolean") {
        return undefined;
    }
    const trimmed = String(value).trim();
    return trimmed === "" ? undefined : trimmed;
}

/**
 * Reads an optional `YYYY-MM-DD` search param.
 *
 * Date filters live in the URL as plain calendar days, not instants: a day is
 * what the operator picked and what makes a shared link readable. Turning it
 * into an absolute range is the caller's job, at the point where the viewer's
 * timezone is known (see notificationFiltersToParams).
 */
export function optionalDaySearch(value: unknown): string | undefined {
    if (typeof value !== "string") return undefined;
    if (!/^\d{4}-\d{2}-\d{2}$/.test(value)) return undefined;

    // Reject a well-shaped but impossible day (e.g. 2026-02-31) — Date rolls it
    // over silently, which would quietly filter on a day nobody asked for.
    const parsed = new Date(`${value}T00:00:00`);
    if (Number.isNaN(parsed.getTime())) return undefined;
    if (formatDay(parsed) !== value) return undefined;

    return value;
}

/** Formats a Date as the `YYYY-MM-DD` day it falls on in the LOCAL timezone. */
export function formatDay(d: Date): string {
    const month = `${d.getMonth() + 1}`.padStart(2, "0");
    const day = `${d.getDate()}`.padStart(2, "0");
    return `${d.getFullYear()}-${month}-${day}`;
}
