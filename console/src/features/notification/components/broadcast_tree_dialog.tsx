import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogHeader,
    DialogTitle,
    ErrorMessage,
    Loading,
    Separator,
    formatDate,
} from "netra";
import { useState } from "react";

import { DeliveryTreeView } from "@/features/notification/components/delivery_tree";
import { StatusTag } from "@/components/status_tag";
import { useBroadcastDeliveryTree } from "@/features/notification/notification_hooks";
import { useGetProjectIDFromParams } from "@/features/project/project_hooks";
import { BroadcastListItem } from "@/features/notification/notification_types";
import { targetToString } from "@/lib/utils";

interface BroadcastTreeDialogProps {
    broadcast: BroadcastListItem;
    open: boolean;
    setOpen: (open: boolean) => void;
}

// BroadcastTreeDialog answers "what happened to this broadcast?" — the fan-out
// from audience down to per-status outcomes.
//
// The tree is fetched HERE rather than on the list, because it is an aggregate
// over every notification in the broadcast and has no business riding every list
// refetch. `open` gates the query, same split as DeliveryDetailDialog.
export function BroadcastTreeDialog({
    broadcast,
    open,
    setOpen,
}: BroadcastTreeDialogProps) {
    const projectID = useGetProjectIDFromParams();

    const { data, isLoading, isError } = useBroadcastDeliveryTree(
        projectID,
        broadcast.id,
        open
    );

    const tree = data?.data;

    return (
        <Dialog open={open} onOpenChange={setOpen}>
            {/* `sm:max-w-2xl!` needs the `!`: netra's stylesheet is imported
                after the console's Tailwind, so it wins on source order at equal
                specificity. See the note in agent-docs/overview.md. */}
            <DialogContent className="max-h-[85vh] overflow-y-auto sm:max-w-2xl!">
                <DialogHeader>
                    <DialogTitle>Broadcast delivery</DialogTitle>
                    <DialogDescription>
                        How this broadcast fanned out, from audience to
                        per-medium outcome.
                    </DialogDescription>
                </DialogHeader>

                <div className="space-y-1 text-sm">
                    <div className="flex-x flex-wrap items-center gap-2">
                        <span className="text-text-muted">Target</span>
                        <span className="text-text-primary select-text! font-medium">
                            {targetToString(broadcast.target)}
                        </span>
                        <StatusTag status={broadcast.status} />
                    </div>
                    <div className="flex-x flex-wrap items-center gap-2">
                        <span className="text-text-muted">Sent</span>
                        <span className="text-text-primary">
                            {formatDate(new Date(broadcast.created_at), {
                                time: true,
                            })}
                        </span>
                    </div>
                </div>

                <Separator />

                {isLoading && <Loading />}
                {isError && (
                    <ErrorMessage errorMsg="Error loading delivery breakdown" />
                )}
                {tree && <DeliveryTreeView tree={tree} />}
            </DialogContent>
        </Dialog>
    );
}

// BroadcastTreeCell is the row-level trigger, mirroring DeliveryDetailCell on the
// direct-notification table so both kinds are opened the same way.
export function BroadcastTreeCell({
    broadcast,
}: {
    broadcast: BroadcastListItem;
}) {
    const [open, setOpen] = useState(false);

    return (
        <>
            <button
                type="button"
                onClick={() => setOpen(true)}
                className="text-text-muted hover:text-text-primary cursor-pointer text-xs underline underline-offset-2"
            >
                Details
            </button>

            <BroadcastTreeDialog
                broadcast={broadcast}
                open={open}
                setOpen={setOpen}
            />
        </>
    );
}
