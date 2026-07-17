import {
    DEFAULT_NOTIFICATION_KIND,
    EMAIL_FILTERS,
    ListNotificationsPayload,
    NOTIFICATION_KINDS,
    NOTIFICATION_STATUSES,
    NotificationFilters,
} from "@/features/notification/notification_types";
import {
    formatDay,
    optionalDaySearch,
    optionalEnumSearch,
    optionalStringSearch,
} from "@/lib/search";

/**
 * Reads the notifications route's URL search into a filter selection.
 *
 * `kind` always resolves to a concrete value (the table below it has to render
 * something); every filter is optional and an unrecognized one is dropped rather
 * than defaulted — absent means "don't narrow", which is a real answer.
 */
export function validateNotificationSearch(
    search: Record<string, unknown>
): NotificationFilters {
    return {
        kind:
            optionalEnumSearch(search.kind, NOTIFICATION_KINDS) ??
            DEFAULT_NOTIFICATION_KIND,
        status: optionalEnumSearch(search.status, NOTIFICATION_STATUSES),
        email: optionalEnumSearch(search.email, EMAIL_FILTERS),
        channel: optionalStringSearch(search.channel),
        topic: optionalStringSearch(search.topic),
        event: optionalStringSearch(search.event),
        recipient_search: optionalStringSearch(search.recipient_search),
        from: optionalDaySearch(search.from),
        to: optionalDaySearch(search.to),
    };
}

/**
 * How many filters are narrowing the list — i.e. what the "Clear" button would
 * clear. `kind` is excluded: it selects which table you are looking at, not how
 * much of it you are hiding, and Clear must not switch tables under you.
 */
export function activeFilterCount(filters: NotificationFilters): number {
    return Object.entries(filters).filter(
        ([key, value]) => key !== "kind" && value !== undefined
    ).length;
}

/**
 * Turns a picked calendar day into the instant that day STARTS in the viewer's
 * own timezone. `new Date("2026-07-10T00:00:00")` — no trailing Z — is parsed as
 * local time, which is exactly the intent: "the 10th" means the operator's 10th.
 */
function startOfLocalDay(day: string): Date {
    return new Date(`${day}T00:00:00`);
}

/**
 * The last instant of a picked day, local. Built as start-of-next-day minus 1ms
 * rather than hardcoding 23:59:59.999, so it stays correct across a DST
 * transition (a 23- or 25-hour day still ends where the next one begins).
 */
function endOfLocalDay(day: string): Date {
    const next = startOfLocalDay(day);
    next.setDate(next.getDate() + 1);
    return new Date(next.getTime() - 1);
}

/**
 * Maps the filter selection to the list endpoint's query params.
 *
 * Every `undefined` is dropped rather than sent blank — the API decodes
 * `created_from` into a *time.Time, so an empty `?created_from=` is a hard 400,
 * not an ignored param.
 */
export function notificationFiltersToParams(
    filters: NotificationFilters
): Partial<ListNotificationsPayload> {
    const params: Partial<ListNotificationsPayload> = {};

    if (filters.status) params.status = filters.status;
    if (filters.email) params.email = filters.email;
    if (filters.channel) params.channel = filters.channel;
    if (filters.topic) params.topic = filters.topic;
    if (filters.event) params.event = filters.event;
    if (filters.recipient_search) {
        params.recipient_search = filters.recipient_search;
    }
    if (filters.from) {
        params.created_from = startOfLocalDay(filters.from).toISOString();
    }
    if (filters.to) {
        params.created_to = endOfLocalDay(filters.to).toISOString();
    }

    return params;
}

/** The picked range as Dates, for netra's range DatePicker. */
export function filterRangeToDates(filters: NotificationFilters): Date[] {
    const dates: Date[] = [];
    if (filters.from) dates.push(startOfLocalDay(filters.from));
    if (filters.to) dates.push(startOfLocalDay(filters.to));
    return dates;
}

/**
 * The DatePicker's selection as `from`/`to` days.
 *
 * A half-picked range (one click so far) becomes an OPEN-ENDED `from` with no
 * `to` — "on or after that day" — and NOT a single-day `from == to`.
 *
 * That is load-bearing, not a nicety. netra's range DatePicker wraps
 * @rehookify/datepicker, where a two-date selection means "this range is
 * complete": clicking again starts a NEW one. Since filterRangeToDates feeds
 * this state straight back in, closing the range on the first click made the
 * second click start over — so a range could never be picked at all, only the
 * last day clicked. An open `from` keeps the selection mid-pick, so the second
 * click completes it.
 */
export function datesToFilterRange(dates: Date[]): {
    from: string | undefined;
    to: string | undefined;
} {
    return {
        from: dates[0] ? formatDay(dates[0]) : undefined,
        to: dates[1] ? formatDay(dates[1]) : undefined,
    };
}
