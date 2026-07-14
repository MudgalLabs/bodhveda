import { FC, ReactNode, useEffect, useMemo, useState } from "react";
import { useGetProjectIDFromParams } from "@/features/project/project_hooks";
import {
    Alert,
    Button,
    Dialog,
    DialogContent,
    DialogFooter,
    DialogHeader,
    DialogTitle,
    DialogTrigger,
    IconBadgeInfo,
    Input,
    Label,
    toast,
    ToggleGroup,
    ToggleGroupItem,
    WithLabel,
} from "netra";
import { useCreateProjectPreference } from "@/features/preference/preference_hooks";
import {
    PREFERENCE_MEDIUMS,
    PREFERENCE_MEDIUM_LABELS,
    PreferenceMedium,
} from "@/features/preference/preference_type";
import { apiErrorHandler } from "@/lib/api";

interface CreateProjectPreferenceModalProps {
    renderTrigger: () => ReactNode;
}

export const CreateProjectPreferenceModal: FC<
    CreateProjectPreferenceModalProps
> = ({ renderTrigger }) => {
    const projectID = useGetProjectIDFromParams();

    const [open, setOpen] = useState(false);
    const [label, setLabel] = useState("");
    const [defaultEnabled, setDefaultEnabled] = useState(true);
    const [channel, setChannel] = useState("");
    const [event, setEvent] = useState("");
    const [topic, setTopic] = useState("");
    const [mediums, setMediums] = useState<PreferenceMedium[]>(["in_app"]);

    // One catalog row is created per selected medium; the backend stores a
    // preference per (target, medium).
    const { mutateAsync: create, isPending } = useCreateProjectPreference();

    const disableCreate =
        !label.trim() || !channel.trim() || mediums.length === 0;

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();

        if (disableCreate) return;

        try {
            await Promise.all(
                mediums.map((medium) =>
                    create({
                        projectID,
                        payload: {
                            label: label.trim(),
                            default_enabled: defaultEnabled,
                            channel: channel.trim(),
                            event: event.trim() || null,
                            topic: topic.trim() || null,
                            medium,
                        },
                    })
                )
            );

            toast.success(`Preference "${label}" created successfully`);
            setOpen(false);
        } catch (err) {
            apiErrorHandler(err);
        }
    };

    const targetFormatted = useMemo(() => {
        let str = "";
        if (channel.trim()) str += channel.trim();
        if (topic.trim()) str += ` : ${topic.trim()}`;
        if (event.trim()) str += ` : ${event.trim()}`;
        return str;
    }, [channel, event, topic]);

    useEffect(() => {
        if (open) {
            setLabel("");
            setDefaultEnabled(true);
            setChannel("");
            setEvent("");
            setTopic("");
            setMediums(["in_app"]);
        }
    }, [open]);

    return (
        <Dialog open={open} onOpenChange={setOpen}>
            <DialogTrigger asChild>{renderTrigger()}</DialogTrigger>
            <DialogContent>
                <DialogHeader>
                    <DialogTitle>Create Preference</DialogTitle>
                </DialogHeader>

                <p>
                    Create a project level preference that will be applied to
                    all recipients in this project. You should allow recipients
                    to override project level preferences in their notification
                    settings.
                </p>

                <Alert>
                    <IconBadgeInfo />
                    <p className="text-text-muted">
                        Recipient preferences are accessible via the{" "}
                        <a
                            href="https://docs.bodhveda.com/api-reference/endpoint/recipients/preferences/list-preferences"
                            target="_blank"
                            rel="noopener noreferrer"
                        >
                            API
                        </a>
                        .
                    </p>
                </Alert>
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

                    <WithLabel Label={<Label>Mediums</Label>}>
                        <ToggleGroup
                            className="[&_*]:h-8 pl-0!"
                            type="multiple"
                            size="small"
                            value={mediums}
                            onValueChange={(value: string[]) =>
                                setMediums(value as PreferenceMedium[])
                            }
                        >
                            {PREFERENCE_MEDIUMS.map((medium) => (
                                <ToggleGroupItem
                                    key={medium}
                                    value={medium}
                                    className="whitespace-nowrap"
                                >
                                    {PREFERENCE_MEDIUM_LABELS[medium]}
                                </ToggleGroupItem>
                            ))}
                        </ToggleGroup>
                    </WithLabel>

                    <WithLabel Label={<Label>Channel</Label>}>
                        <Input
                            className="w-full!"
                            placeholder="posts"
                            required
                            maxLength={256}
                            value={channel}
                            onChange={(e) => setChannel(e.target.value)}
                        />
                    </WithLabel>

                    <WithLabel Label={<Label>Topic</Label>}>
                        <Input
                            className="w-full!"
                            placeholder="'any' / 'none' / anything_but_any_or_none"
                            required
                            maxLength={256}
                            value={topic}
                            onChange={(e) => setTopic(e.target.value)}
                        />
                    </WithLabel>

                    <WithLabel Label={<Label>Event</Label>}>
                        <Input
                            className="w-full!"
                            placeholder="new_comment"
                            required
                            maxLength={256}
                            value={event}
                            onChange={(e) => setEvent(e.target.value)}
                        />
                    </WithLabel>

                    <DialogFooter className="flex-x justify-between!">
                        <div>
                            {targetFormatted && (
                                <p>
                                    <strong>Target:</strong> {targetFormatted}
                                </p>
                            )}
                        </div>
                        <Button
                            type="submit"
                            disabled={disableCreate}
                            loading={isPending}
                        >
                            Create
                        </Button>
                    </DialogFooter>
                </form>
            </DialogContent>
        </Dialog>
    );
};
