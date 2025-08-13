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
import { ProjectPreference } from "@/features/preference/preference_type";
import { useDeleteProjectPreference } from "../preference_hooks";

interface DeleteProjectPreferenceModalProps {
    open: boolean;
    setOpen: (open: boolean) => void;
    projectID: string;
    preference: ProjectPreference;
}

export function DeleteProjectPreferenceModal(
    props: DeleteProjectPreferenceModalProps
) {
    const { open, setOpen, projectID, preference } = props;

    const [confirmText, setConfirmText] = useState("");

    const { mutate: deletePreference, isPending: isDeleting } =
        useDeleteProjectPreference(projectID, {
            onSuccess: () => {
                toast.success(
                    `Preference ${preference.label} deleted successfully`
                );
                setOpen(false);
            },
        });

    const canDelete = confirmText === "DELETE";

    const handleSubmit = (e: React.FormEvent) => {
        e.preventDefault();
        if (!canDelete) return;

        deletePreference({ preferenceID: preference.id });
    };

    return (
        <Dialog open={open} onOpenChange={setOpen}>
            <DialogContent>
                <DialogHeader>
                    <DialogTitle>Delete Preference</DialogTitle>
                </DialogHeader>

                <p>
                    Are you sure you want to delete the{" "}
                    <span className="font-bold text-text-primary">
                        {preference.label}
                    </span>{" "}
                    preference?
                </p>

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
                            Delete Preference
                        </Button>
                    </DialogFooter>
                </form>
            </DialogContent>
        </Dialog>
    );
}
