import { useEffect, useState } from "react";
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
    ToggleGroup,
    ToggleGroupItem,
    WithLabel,
} from "netra";
import { useUpdateProjectPreference } from "@/features/preference/preference_hooks";
import {
    mediumLabel,
    ProjectPreference,
} from "@/features/preference/preference_type";
import { apiErrorHandler } from "@/lib/api";
import { targetToString } from "@/lib/utils";

interface EditProjectPreferenceModalProps {
    open: boolean;
    setOpen: (open: boolean) => void;
    projectID: string;
    preference: ProjectPreference;
}

// Only label and the project-level default are editable. The target
// (channel/topic/event) and medium form the immutable natural key, so they are
// shown read-only for context — changing them would be a delete + create.
export function EditProjectPreferenceModal(
    props: EditProjectPreferenceModalProps
) {
    const { open, setOpen, projectID, preference } = props;

    const [label, setLabel] = useState(preference.label);
    const [defaultEnabled, setDefaultEnabled] = useState(
        preference.default_enabled
    );

    const { mutateAsync: update, isPending } = useUpdateProjectPreference(
        projectID,
        {
            onSuccess: () => {
                toast.success(`Preference "${label}" updated successfully`);
                setOpen(false);
            },
        }
    );

    // Reset the form to the preference's current values each time it opens, so a
    // cancelled edit doesn't leak into the next one.
    useEffect(() => {
        if (open) {
            setLabel(preference.label);
            setDefaultEnabled(preference.default_enabled);
        }
    }, [open, preference.label, preference.default_enabled]);

    const disableSave = !label.trim();

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        if (disableSave) return;

        try {
            await update({
                preferenceID: preference.id,
                payload: {
                    label: label.trim(),
                    default_enabled: defaultEnabled,
                },
            });
        } catch (err) {
            apiErrorHandler(err);
        }
    };

    return (
        <Dialog open={open} onOpenChange={setOpen}>
            <DialogContent>
                <DialogHeader>
                    <DialogTitle>Edit Preference</DialogTitle>
                </DialogHeader>

                <p>
                    Update the label and default for this preference. The medium
                    and target can't be changed — create a new preference for a
                    different target.
                </p>

                <form className="flex flex-col gap-4" onSubmit={handleSubmit}>
                    <WithLabel Label={<Label>Label</Label>}>
                        <Input
                            className="w-full!"
                            placeholder="Comments on photos of you"
                            required
                            maxLength={256}
                            value={label}
                            onChange={(e) => setLabel(e.target.value)}
                        />
                    </WithLabel>

                    <WithLabel Label={<Label>Default</Label>}>
                        <ToggleGroup
                            className="[&_*]:h-8 pl-0!"
                            type="single"
                            size="small"
                            value={defaultEnabled ? "enabled" : "disabled"}
                            onValueChange={(value) =>
                                value && setDefaultEnabled(value === "enabled")
                            }
                        >
                            <ToggleGroupItem value="enabled">
                                Enabled
                            </ToggleGroupItem>

                            <ToggleGroupItem value="disabled">
                                Disabled
                            </ToggleGroupItem>
                        </ToggleGroup>
                    </WithLabel>

                    <WithLabel Label={<Label>Medium</Label>}>
                        <p className="text-text-muted">
                            {mediumLabel(preference.medium)}
                        </p>
                    </WithLabel>

                    <WithLabel Label={<Label>Target</Label>}>
                        <p className="text-text-muted">
                            {targetToString(preference.target)}
                        </p>
                    </WithLabel>

                    <DialogFooter>
                        <Button
                            variant="secondary"
                            type="button"
                            onClick={() => setOpen(false)}
                        >
                            Cancel
                        </Button>

                        <Button
                            type="submit"
                            disabled={disableSave}
                            loading={isPending}
                        >
                            Save
                        </Button>
                    </DialogFooter>
                </form>
            </DialogContent>
        </Dialog>
    );
}
