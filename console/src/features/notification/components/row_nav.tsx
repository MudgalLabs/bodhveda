// Full-row navigation for the notification tables.
//
// netra's DataTable renders its own <tr> and exposes no onRowClick, so the row
// cannot take a handler. Instead one cell renders an absolutely-positioned <Link>
// covering the whole row, and ROW_NAV_TABLE_CLASS makes the <tr> the positioning
// context.
//
// Why this shape rather than the obvious alternatives:
//
//   - Wrapping every cell's content in its own <Link> would give one tab stop PER
//     CELL — eight per row here — which makes keyboard traversal of a paginated
//     table miserable.
//   - An onClick on each cell would have no keyboard path at all, and would break
//     middle-click / cmd-click to open in a new tab, which is exactly how you use
//     a debugging table.
//
// One real <Link> per row gets all three right: whole-row hit area, a single tab
// stop, and normal browser link behaviour.
//
// ⚠️ These are CLASS CONSTANTS rather than a <RowNavLink> wrapper component.
// TanStack Router types `to` and `params` together per route, so a wrapper taking
// `to: string` + `params: Record<string, string>` throws away that checking and
// fails to compile. Each cell builds its own typed <Link> and borrows the styling.

/**
 * Class for the element wrapping a DataTable whose rows should navigate.
 * Establishes the positioning context on each row and shows a pointer.
 *
 * The `[&_tbody_tr]` arbitrary selector is deliberate: netra's stylesheet loads
 * after the console's Tailwind and wins on source order at equal specificity, so
 * a plain utility class on the row would lose. See agent-docs/overview.md.
 */
export const ROW_NAV_TABLE_CLASS =
    "[&_tbody_tr]:relative [&_tbody_tr]:cursor-pointer";

/**
 * Class for the invisible full-row link. Put it on exactly ONE <Link> per row,
 * in a `nav` column with no header. Give the link an `aria-label` that identifies
 * the row rather than saying "open".
 */
export const ROW_NAV_LINK_CLASS =
    "focus-visible:ring-focus-ring absolute inset-0 z-0 rounded-sm focus-visible:ring-2 focus-visible:outline-none";

/**
 * RowNavShield raises interactive cell content above the row-wide link so it stays
 * clickable. Without it, a recipient link inside a navigating row is unreachable.
 */
export function RowNavShield({ children }: { children: React.ReactNode }) {
    return <span className="relative z-10">{children}</span>;
}
