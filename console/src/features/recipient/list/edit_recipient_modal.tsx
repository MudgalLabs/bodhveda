import { FC, useEffect, useRef, useState } from "react";
import isEqual from "lodash/isEqual";

import { useGetProjectIDFromParams } from "@/features/project/project_hooks";
import {
    Button,
    Dialog,
    DialogContent,
    DialogFooter,
    DialogHeader,
    DialogTitle,
    Input,
    Label,
    toast,
    WithLabel,
} from "netra";
import { useEditRecipient } from "@/features/recipient/recipient_hooks";
import { apiErrorHandler } from "@/lib/api";
import { RecipientListItem } from "@/features/recipient/recipient_types";

interface EditRecipientModalProps {
    recipient: RecipientListItem;
    open: boolean;
    setOpen: (open: boolean) => void;
}

export const EditRecipientModal: FC<EditRecipientModalProps> = ({
    recipient,
    open,
    setOpen,
}) => {
    const projectID = useGetProjectIDFromParams();

    const [state, setState] = useState(recipient);
    const initialState = useRef(recipient);

    const updateField = (field: keyof RecipientListItem, value: string) => {
        setState((prev) => ({ ...prev, [field]: value }));
    };

    const { mutate: update, isPending } = useEditRecipient(
        projectID,
        recipient.id,
        {
            onSuccess: () => {
                toast.success(`Recipient ${recipient.id} updated successfully`);
                setOpen(false);
            },
            onError: apiErrorHandler,
        }
    );

    const handleSubmit = (e: React.FormEvent) => {
        e.preventDefault();

        if (!state.name.trim()) return;

        update({
            payload: {
                name: state.name.trim() || null,
            },
        });
    };

    const disableCreate = isEqual(initialState.current, state);

    useEffect(() => {
        if (open) {
            setState(recipient);
        }
    }, [open, recipient]);

    return (
        <Dialog open={open} onOpenChange={setOpen}>
            <DialogContent>
                <DialogHeader>
                    <DialogTitle>Edit Recipient</DialogTitle>
                </DialogHeader>
                <form className="flex flex-col gap-4" onSubmit={handleSubmit}>
                    <WithLabel Label={<Label>Name</Label>}>
                        <Input
                            className="w-full!"
                            placeholder="John Doe"
                            type="text"
                            maxLength={256}
                            value={state.name}
                            onChange={(e) =>
                                updateField("name", e.target.value)
                            }
                        />
                    </WithLabel>
                    <DialogFooter>
                        <Button
                            type="submit"
                            disabled={disableCreate}
                            loading={isPending}
                        >
                            Save
                        </Button>
                    </DialogFooter>
                </form>
            </DialogContent>
        </Dialog>
    );
};
