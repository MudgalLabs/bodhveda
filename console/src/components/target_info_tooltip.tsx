import { Tooltip } from "netra";
import { ReactNode } from "react";

interface TargetInfoTooltip {
    children: ReactNode;
}

export function TargetInfoTooltip(props: TargetInfoTooltip) {
    const { children } = props;

    return (
        <Tooltip
            content={
                <div>
                    <p>channel : topic : event</p>
                </div>
            }
            contentProps={{
                align: "center",
            }}
        >
            {children}
        </Tooltip>
    );
}
