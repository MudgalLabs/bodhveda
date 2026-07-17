import { useEffect, useState } from "react";
import { Button, DatePicker, Input, Select, formatDate } from "netra";

import {
    EMAIL_DELIVERY_FILTER_STATUSES,
    EmailFilter,
    NOTIFICATION_STATUSES,
    NotificationFilters,
    NotificationStatus,
} from "@/features/notification/notification_types";
import {
    activeFilterCount,
    datesToFilterRange,
    filterRangeToDates,
} from "@/features/notification/notification_filters";

interface NotificationFilterBarProps {
    filters: NotificationFilters;
    onChange: (next: NotificationFilters) => void;
}

// netra's Select has no "clear" affordance, so an explicit "any" option is the
// only way back out of a chosen value. It carries a sentinel rather than "",
// because Radix's Select reserves the empty string for its placeholder.
const ANY = "__any__";

const statusOptions = [
    { value: ANY, label: "Any status" },
    ...NOTIFICATION_STATUSES.map((s) => ({
        value: s,
        label: titleCase(s),
    })),
];

const emailOptions = [
    { value: ANY, label: "Any email state" },
    // The medium dimension: was email attempted at all? `none` is how in-app-only
    // sends stay findable — see NotificationFilters.
    { value: "none", label: "No email attempted" },
    { value: "any", label: "Email attempted" },
    ...EMAIL_DELIVERY_FILTER_STATUSES.map((s) => ({
        value: s,
        label: `Email: ${titleCase(s)}`,
    })),
];

function titleCase(s: string): string {
    return s
        .split("_")
        .map((w) => w.charAt(0).toUpperCase() + w.slice(1))
        .join(" ");
}

/**
 * The notifications list's filter controls (Phase 9.4).
 *
 * Every control writes straight through to the URL via `onChange` — there is no
 * "Apply" button and no local mirror of the selection, so what the table shows
 * and what the address bar says cannot drift apart. The one exception is the
 * free-text inputs, which are debounced (see DebouncedInput) so typing doesn't
 * push a history entry and a request per keystroke.
 */
export function NotificationFilterBar({
    filters,
    onChange,
}: NotificationFilterBarProps) {
    const count = activeFilterCount(filters);
    const dates = filterRangeToDates(filters);

    const set = <K extends keyof NotificationFilters>(
        key: K,
        value: NotificationFilters[K]
    ) => onChange({ ...filters, [key]: value });

    return (
        <div className="flex flex-wrap items-center gap-2 mb-4">
            <Select
                options={statusOptions}
                value={filters.status ?? ANY}
                onValueChange={(v) =>
                    set(
                        "status",
                        v === ANY ? undefined : (v as NotificationStatus)
                    )
                }
                placeholder="Any status"
                classNames={{ trigger: "h-8! w-fit! min-w-36!" }}
            />

            <Select
                options={emailOptions}
                value={filters.email ?? ANY}
                onValueChange={(v) =>
                    set("email", v === ANY ? undefined : (v as EmailFilter))
                }
                placeholder="Any email state"
                classNames={{ trigger: "h-8! w-fit! min-w-44!" }}
            />

            <DatePicker
                mode="range"
                dates={dates}
                onDatesChange={(d) => {
                    const { from, to } = datesToFilterRange(d);
                    onChange({ ...filters, from, to });
                }}
                placeholder="Any date"
                className="h-8! w-fit!"
            />

            <DebouncedInput
                value={filters.recipient_search ?? ""}
                onDebouncedChange={(v) =>
                    set("recipient_search", v || undefined)
                }
                placeholder="Search recipient"
                className="h-8! w-40!"
            />

            <DebouncedInput
                value={filters.channel ?? ""}
                onDebouncedChange={(v) => set("channel", v || undefined)}
                placeholder="Channel"
                className="h-8! w-28!"
            />
            <DebouncedInput
                value={filters.topic ?? ""}
                onDebouncedChange={(v) => set("topic", v || undefined)}
                placeholder="Topic"
                className="h-8! w-28!"
            />
            <DebouncedInput
                value={filters.event ?? ""}
                onDebouncedChange={(v) => set("event", v || undefined)}
                placeholder="Event"
                className="h-8! w-28!"
            />

            {count > 0 && (
                <Button
                    variant="ghost"
                    size="small"
                    onClick={() => onChange({ kind: filters.kind })}
                >
                    Clear {count === 1 ? "filter" : `${count} filters`}
                </Button>
            )}

            {(filters.from || filters.to) && (
                <span className="text-xs text-text-muted">
                    {rangeLabel(filters)}
                </span>
            )}
        </div>
    );
}

function rangeLabel(filters: NotificationFilters): string {
    const dates = filterRangeToDates(filters);
    if (dates.length === 0) return "";
    const from = formatDate(dates[0]);
    const to = dates[1] ? formatDate(dates[1]) : from;
    return from === to ? from : `${from} → ${to}`;
}

interface DebouncedInputProps {
    value: string;
    onDebouncedChange: (value: string) => void;
    placeholder?: string;
    className?: string;
}

/**
 * A text input that reports upward only once typing settles.
 *
 * These filters live in the URL, so an un-debounced input would push a router
 * navigation and a refetch per keystroke — and leave the back button walking
 * through every prefix of what was typed.
 *
 * It keeps a local mirror while focused, but re-syncs whenever the prop moves on
 * its own (Clear, or the back button), so the URL stays the source of truth.
 */
function DebouncedInput({
    value,
    onDebouncedChange,
    placeholder,
    className,
}: DebouncedInputProps) {
    const [local, setLocal] = useState(value);

    useEffect(() => {
        // Re-sync only on a GENUINELY external change (Clear, back button), not
        // on the echo of our own debounced write. Since we emit `local.trim()`,
        // the echo comes back trimmed: syncing on it unconditionally would erase
        // a trailing space the moment the debounce fired, moving the caret out
        // from under someone still typing "billing alerts".
        if (value === local.trim()) return;
        setLocal(value);
        // Intentionally keyed on `value` alone: this reacts to the prop moving,
        // and reading `local` here must not re-run it.
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [value]);

    useEffect(() => {
        if (local.trim() === value) return;

        const t = setTimeout(() => onDebouncedChange(local.trim()), 300);
        return () => clearTimeout(t);
        // onDebouncedChange is a fresh closure each render; depending on it would
        // reset the timer every render and never fire.
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [local, value]);

    return (
        <Input
            value={local}
            onChange={(e) => setLocal(e.target.value)}
            placeholder={placeholder}
            className={className}
        />
    );
}
