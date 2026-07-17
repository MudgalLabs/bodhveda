import { Button, DatePicker } from "netra";

import {
    AnalyticsRange,
    DEFAULT_RANGE_PRESET,
    RANGE_PRESETS,
    RANGE_PRESET_LABELS,
    RangePreset,
    datesToRange,
    isCustomRange,
    rangeToDates,
} from "@/features/dashboard/analytics_range";

// The analytics range control: relative presets (7/30/90 days) plus a custom
// absolute picker. Selecting a preset clears any custom from/to; picking a
// custom range overrides the preset. Every choice writes straight to the URL via
// onChange (no Apply button, no local mirror) so the address bar and the charts
// can't drift.
export function RangePicker({
    range,
    onChange,
}: {
    range: AnalyticsRange;
    onChange: (next: AnalyticsRange) => void;
}) {
    const custom = isCustomRange(range);
    const dates = rangeToDates(range);

    const selectPreset = (preset: RangePreset) =>
        onChange({ preset, from: undefined, to: undefined });

    return (
        <div className="flex flex-wrap items-center gap-2">
            {RANGE_PRESETS.map((p) => (
                <Button
                    key={p}
                    size="small"
                    variant={!custom && range.preset === p ? "primary" : "ghost"}
                    onClick={() => selectPreset(p)}
                >
                    {RANGE_PRESET_LABELS[p]}
                </Button>
            ))}

            <DatePicker
                mode="range"
                dates={dates}
                onDatesChange={(d) => {
                    const { from, to } = datesToRange(d);
                    // Keep the preset as a fallback for when the custom range is
                    // cleared, but from/to take precedence while both are set.
                    onChange({ preset: range.preset, from, to });
                }}
                placeholder="Custom range"
                className="h-8! w-fit!"
            />

            {custom && (
                <Button
                    variant="ghost"
                    size="small"
                    onClick={() =>
                        onChange({
                            preset: DEFAULT_RANGE_PRESET,
                            from: undefined,
                            to: undefined,
                        })
                    }
                >
                    Reset
                </Button>
            )}
        </div>
    );
}
