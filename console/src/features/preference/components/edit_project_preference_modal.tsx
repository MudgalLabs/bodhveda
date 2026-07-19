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
    Textarea,
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

// Only name, description and the project-level default are editable. The target
// (channel/topic/event) and medium form the immutable natural key, so they are
// shown read-only for context — changing them would be a delete + create.
export function EditProjectPreferenceModal(
    props: EditProjectPreferenceModalProps
) {
    const { open, setOpen, projectID, preference } = props;

    const [name, setName] = useState(preference.name);
    const [description, setDescription] = useState(
        preference.description ?? ""
    );
    const [defaultEnabled, setDefaultEnabled] = useState(
        preference.default_enabled
    );

    const { mutateAsync: update, isPending } = useUpdateProjectPreference(
        projectID,
        {
            onSuccess: () => {
                toast.success(`Preference "${name}" updated successfully`);
                setOpen(false);
            },
        }
    );

    // Reset the form to the preference's current values each time it opens, so a
    // cancelled edit doesn't leak into the next one.
    useEffect(() => {
        if (open) {
            setName(preference.name);
            setDescription(preference.description ?? "");
            setDefaultEnabled(preference.default_enabled);
        }
    }, [
        open,
        preference.name,
        preference.description,
        preference.default_enabled,
    ]);

    const disableSave = !name.trim();

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        if (disableSave) return;

        try {
            await update({
                preferenceID: preference.id,
                payload: {
                    name: name.trim(),
                    description: description.trim() || undefined,
                    default_enabled: defaultEnabled,
                },
            });
        } catch (err) {
            apiErrorHandler(err);
        }
    };

    return (
        <Dialog open={open} onOpenChange={setOpen}>
            {/* Cap the height and let the body scroll so the header (with its
                close button) and the footer stay pinned on short viewports. */}
            <DialogContent className="flex max-h-[90vh] flex-col">
                <DialogHeader>
                    <DialogTitle>Edit Preference</DialogTitle>
                </DialogHeader>

                <form
                    className="flex min-h-0 flex-1 flex-col gap-4"
                    onSubmit={handleSubmit}
                >
                    <div className="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto pr-1">
                        <p>
                            Update the name, description and default for this
                            preference. The medium and target can't be changed —
                            create a new preference for a different target.
                        </p>

                        <WithLabel Label={<Label>Name</Label>}>
                            <Input
                                className="w-full!"
                                placeholder="Comments on photos of you"
                                required
                                maxLength={256}
                                value={name}
                                onChange={(e) => setName(e.target.value)}
                            />
                        </WithLabel>

                        <WithLabel Label={<Label>Description</Label>}>
                            <Textarea
                                className="w-full!"
                                placeholder="Receive notifications about new products, features, and more."
                                maxLength={1024}
                                value={description}
                                onChange={(e) => setDescription(e.target.value)}
                            />
                        </WithLabel>

                        <WithLabel Label={<Label>Default</Label>}>
                            <ToggleGroup
                                className="[&_*]:h-8 pl-0!"
                                type="single"
                                size="small"
                                value={defaultEnabled ? "enabled" : "disabled"}
                                onValueChange={(value) =>
                                    value &&
                                    setDefaultEnabled(value === "enabled")
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
                    </div>

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
