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
import { useState } from "react";
import { APIKey } from "@/features/api_key/api_key_types";
import { useDeleteAPIKey } from "@/features/api_key/api_key_hooks";

interface DeleteAPIKeyModalProps {
    open: boolean;
    setOpen: (open: boolean) => void;
    projectID: string;
    apiKey: APIKey;
}

export function DeleteAPIKeyModal(props: DeleteAPIKeyModalProps) {
    const { open, setOpen, projectID, apiKey } = props;

    const [confirmText, setConfirmText] = useState("");

    const { mutate: deleteKey, isPending: isDeleting } = useDeleteAPIKey(
        projectID,
        {
            onSuccess: () => {
                toast.success(`API Key ${apiKey.name} deleted successfully`);
                setOpen(false);
            },
        }
    );

    const canDelete = confirmText === "DELETE";

    const handleSubmit = (e: React.FormEvent) => {
        e.preventDefault();
        if (!canDelete) return;

        deleteKey({ apiKeyID: apiKey.id });
    };

    return (
        <Dialog open={open} onOpenChange={setOpen}>
            <DialogContent>
                <DialogHeader>
                    <DialogTitle>Delete API Key</DialogTitle>

                    <p>
                        Are you sure you want to delete the{" "}
                        <span className="font-bold text-text-primary">
                            {apiKey.name}
                        </span>{" "}
                        API Key?
                    </p>

                    <p className="text-text-destructive">
                        This action cannot be undone.
                    </p>
                </DialogHeader>
                <form className="flex flex-col gap-4" onSubmit={handleSubmit}>
                    <WithLabel
                        Label={
                            <Label>
                                Type <strong>DELETE</strong> to confirm.
                            </Label>
                        }
                    >
                        <Input
                            className="w-full!"
                            type="text"
                            maxLength={256}
                            value={confirmText}
                            onChange={(e) => setConfirmText(e.target.value)}
                        />
                    </WithLabel>
                    <DialogFooter>
                        <Button variant="secondary">Cancel</Button>

                        <Button
                            variant="destructive"
                            type="submit"
                            disabled={!canDelete}
                            loading={isDeleting}
                        >
                            Delete API Key
                        </Button>
                    </DialogFooter>
                </form>
            </DialogContent>
        </Dialog>
    );
}
