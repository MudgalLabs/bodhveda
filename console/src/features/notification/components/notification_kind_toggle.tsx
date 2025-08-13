import { ToggleGroup, ToggleGroupItem } from "netra";
import { NotificationKind } from "../notification_types";

type NotificationKindToggleProps = {
    kind: NotificationKind;
    setKind: (kind: NotificationKind) => void;
    className?: string;
};

export function NotificationKindToggle({
    kind,
    setKind,
    className = "",
}: NotificationKindToggleProps) {
    return (
        <ToggleGroup
            className={`[&_*]:h-8 pl-0! ${className}`}
            type="single"
            size="small"
            value={kind}
            onValueChange={(value) =>
                value && setKind(value as NotificationKind)
            }
        >
            <ToggleGroupItem value="direct">Direct</ToggleGroupItem>
            <ToggleGroupItem value="broadcast">Broadcast</ToggleGroupItem>
        </ToggleGroup>
    );
}
