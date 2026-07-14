import { useCallback, useEffect, useMemo, useState } from "react";
import {
    Button,
    cn,
    Dialog,
    DialogContent,
    DialogHeader,
    DialogTitle,
    DialogTrigger,
    IconArrowLeft,
    IconArrowRight,
    IconInfo,
    IconSend,
    Input,
    Label,
    MultiStep,
    Switch,
    Textarea,
    toast,
    Tooltip,
    WithLabel,
} from "netra";

import { useGetProjectIDFromParams } from "@/features/project/project_hooks";
import { NotificationKind, SendNotificationResult } from "../notification_types";
import { NotificationKindToggle } from "./notification_kind_toggle";
import { useSendNotification } from "../notification_hooks";
import { apiErrorHandler, APIRes } from "@/lib/api";

type SendNotificationModalProps = {
    renderTrigger: () => React.ReactNode;
};

// Flattened state for easier management.
interface State {
    recipient_id: string;
    channel: string;
    topic: string;
    event: string;
    payload: string;
    // Optional email block (direct sends only). email_enabled gates whether an
    // `email` block is sent at all — presence of the block is the "email
    // eligible" signal on the backend.
    email_enabled: boolean;
    email_subject: string;
    email_html: string;
    email_text: string;
}

const INITIAL_STATE: State = {
    recipient_id: "",
    channel: "",
    topic: "",
    event: "",
    payload: "",
    email_enabled: false,
    email_subject: "",
    email_html: "",
    email_text: "",
};

export function SendNotificationModal({
    renderTrigger,
}: SendNotificationModalProps) {
    const projectID = useGetProjectIDFromParams();

    const [open, setOpen] = useState(false);
    const [kind, setKind] = useState<NotificationKind>("direct");
    const isBroadcast = kind === "broadcast";

    const [state, setState] = useState<State>(INITIAL_STATE);

    // Only direct sends can carry an email block (email is direct-only).
    const emailEnabled = !isBroadcast && state.email_enabled;

    const { mutate: sendNotification, isPending: isSending } =
        useSendNotification(projectID, {
            onSuccess: (res: APIRes<SendNotificationResult>) => {
                notifyEmailOutcome(res);
                setOpen(false);
                setState(INITIAL_STATE);
            },
            onError: apiErrorHandler,
        });

    const handleSendNotification = () => {
        let target = null;

        if (isBroadcast || state.channel || state.topic || state.event) {
            target = {
                channel: state.channel,
                topic: state.topic,
                event: state.event,
            };
        }

        try {
            const parsedPayload = JSON.parse(state.payload);

            sendNotification({
                recipient_id: state.recipient_id ? state.recipient_id : null,
                target,
                payload: parsedPayload,
                email: emailEnabled
                    ? {
                          subject: state.email_subject,
                          // Omit empty fields; the backend derives text from html
                          // when text is absent.
                          html: state.email_html || undefined,
                          text: state.email_text || undefined,
                      }
                    : undefined,
            });
        } catch {
            toast.error("Payload must be a valid JSON");
        }
    };

    const disablePayloadButton = useMemo(() => {
        const disable = false;

        if (!isBroadcast) {
            if (!state.recipient_id) {
                return true;
            }
        } else {
            if (!state.channel || !state.topic || !state.event) {
                return true;
            }
        }

        return disable;
    }, [isBroadcast, state]);

    const disableSendButton = useMemo(() => {
        if (disablePayloadButton) {
            return true;
        }

        if (state.payload.trim() === "") {
            return true;
        }

        // When the email block is on, it needs a subject and at least one body.
        if (emailEnabled) {
            if (state.email_subject.trim() === "") {
                return true;
            }
            if (
                state.email_html.trim() === "" &&
                state.email_text.trim() === ""
            ) {
                return true;
            }
        }

        return false;
    }, [disablePayloadButton, state, emailEnabled]);

    useEffect(() => {
        if (!open) {
            setKind("direct");
            setState(INITIAL_STATE);
        }
    }, [open]);

    return (
        <Dialog open={open} onOpenChange={setOpen}>
            <DialogTrigger asChild>{renderTrigger()}</DialogTrigger>
            <DialogContent className="sm:max-w-4xl!">
                <DialogHeader>
                    <DialogTitle>Send Notification</DialogTitle>
                </DialogHeader>

                <div className="h-2" />

                <MultiStep.Root>
                    <MultiStep.StepperContainer>
                        <MultiStep.Stepper>
                            {({ index, currentStepIndex }) => {
                                return (
                                    <Tooltip
                                        content={
                                            index === 0 ? "Target" : "Content"
                                        }
                                    >
                                        <div
                                            className={cn(
                                                "h-2 w-8 rounded-md transition-all bg-red-300",
                                                {
                                                    "bg-secondary":
                                                        index >
                                                        currentStepIndex,
                                                    "bg-primary":
                                                        index <=
                                                        currentStepIndex,
                                                    "w-24":
                                                        index ===
                                                        currentStepIndex,
                                                }
                                            )}
                                        />
                                    </Tooltip>
                                );
                            }}
                        </MultiStep.Stepper>
                    </MultiStep.StepperContainer>

                    <div className="h-2" />

                    <MultiStep.Content>
                        <MultiStep.Step id="target-step">
                            <TargetStep
                                state={state}
                                setState={setState}
                                kind={kind}
                                setKind={setKind}
                                isBroadcast={isBroadcast}
                            />
                        </MultiStep.Step>

                        <MultiStep.Step id="payload-step">
                            <PayloadStep
                                state={state}
                                setState={setState}
                                isBroadcast={isBroadcast}
                            />
                        </MultiStep.Step>
                    </MultiStep.Content>

                    <div className="h-4" />

                    <div className="flex w-full justify-between gap-x-4">
                        <MultiStep.PreviousStepButton>
                            {(props) =>
                                props.hasPrevious ? (
                                    <Button
                                        variant="secondary"
                                        onClick={() => props.prev()}
                                        disabled={isSending}
                                    >
                                        <IconArrowLeft />
                                        Target
                                    </Button>
                                ) : (
                                    <span />
                                )
                            }
                        </MultiStep.PreviousStepButton>

                        <MultiStep.NextStepButton>
                            {(props) =>
                                props.hasNext ? (
                                    <Tooltip
                                        disabled={!disablePayloadButton}
                                        content="Some required fields are missing"
                                    >
                                        <Button
                                            variant="primary"
                                            onClick={() => props.next()}
                                            disabled={disablePayloadButton}
                                        >
                                            Content
                                            <IconArrowRight />
                                        </Button>
                                    </Tooltip>
                                ) : (
                                    <Tooltip
                                        disabled={!disableSendButton}
                                        content="Some required fields are missing"
                                    >
                                        <Button
                                            variant="primary"
                                            onClick={handleSendNotification}
                                            loading={isSending}
                                            disabled={disableSendButton}
                                        >
                                            <IconSend />
                                            Send Notification
                                        </Button>
                                    </Tooltip>
                                )
                            }
                        </MultiStep.NextStepButton>
                    </div>
                </MultiStep.Root>
            </DialogContent>
        </Dialog>
    );
}

function TargetStep({
    state,
    setState,
    kind,
    setKind,
    isBroadcast,
}: {
    state: State;
    setState: React.Dispatch<React.SetStateAction<State>>;
    kind: NotificationKind;
    setKind: React.Dispatch<React.SetStateAction<NotificationKind>>;
    isBroadcast: boolean;
}) {
    return (
        <div className="space-y-4">
            <WithLabel Label={<Label required>Notification kind</Label>}>
                <NotificationKindToggle kind={kind} setKind={setKind} />
            </WithLabel>

            {!isBroadcast && (
                <WithLabel Label={<Label required>Recipient ID</Label>}>
                    <Input
                        type="text"
                        className="w-full!"
                        placeholder="Recipient's unique ID"
                        value={state.recipient_id}
                        onChange={(e) =>
                            setState((prev) => ({
                                ...prev,
                                recipient_id: e.target.value,
                            }))
                        }
                    />
                </WithLabel>
            )}

            <WithLabel Label={<Label required={isBroadcast}>Channel</Label>}>
                <Input
                    className="w-full!"
                    placeholder="posts"
                    required
                    maxLength={256}
                    value={state.channel}
                    onChange={(e) =>
                        setState((prev) => ({
                            ...prev,
                            channel: e.target.value,
                        }))
                    }
                />
            </WithLabel>

            <WithLabel Label={<Label required={isBroadcast}>Topic</Label>}>
                <Input
                    className="w-full!"
                    placeholder="'any' / 'none' / anything_but_any_or_none"
                    required
                    maxLength={256}
                    value={state.topic}
                    onChange={(e) =>
                        setState((prev) => ({
                            ...prev,
                            topic: e.target.value,
                        }))
                    }
                />
            </WithLabel>

            <WithLabel Label={<Label required={isBroadcast}>Event</Label>}>
                <Input
                    className="w-full!"
                    placeholder="new_comment"
                    required
                    maxLength={256}
                    value={state.event}
                    onChange={(e) =>
                        setState((prev) => ({
                            ...prev,
                            event: e.target.value,
                        }))
                    }
                />
            </WithLabel>
        </div>
    );
}

function PayloadStep({
    state,
    setState,
    isBroadcast,
}: {
    state: State;
    setState: React.Dispatch<React.SetStateAction<State>>;
    isBroadcast: boolean;
}) {
    const placeholder = `{
    "key": "value"
}`;
    const isPayloadValidJSON = useMemo(() => {
        if (state.payload.trim() === "") {
            return true;
        }

        try {
            JSON.parse(state.payload);
            return true;
        } catch {
            return false;
        }
    }, [state.payload]);

    const beautifyJSON = useCallback(() => {
        try {
            const parsed = JSON.parse(state.payload);
            const beautified = JSON.stringify(parsed, null, 4);
            setState((prev) => ({
                ...prev,
                payload: beautified,
            }));
        } catch {
            // Do nothing if invalid JSON.
        }
    }, [state.payload, setState]);

    return (
        <div className="space-y-6 max-h-[55vh] overflow-y-auto pr-1">
            <WithLabel
                Label={
                    <span className="flex-x justify-between">
                        <span className="flex-x">
                            <Label required>Payload</Label>
                            <Tooltip
                                content={
                                    <>
                                        <p>
                                            The in-app payload to send with the
                                            notification.
                                        </p>
                                        <p>Must be valid JSON.</p>
                                    </>
                                }
                            >
                                <IconInfo />
                            </Tooltip>
                        </span>

                        <Button
                            variant="ghost"
                            size="small"
                            onClick={beautifyJSON}
                        >
                            Beautify
                        </Button>
                    </span>
                }
            >
                <Textarea
                    className="w-full! h-48"
                    placeholder={placeholder}
                    value={state.payload}
                    onChange={(e) =>
                        setState((prev) => ({
                            ...prev,
                            payload: e.target.value,
                        }))
                    }
                    error={!isPayloadValidJSON}
                    errorMsg="Payload is not a valid JSON"
                />
            </WithLabel>

            {!isBroadcast && (
                <EmailSection state={state} setState={setState} />
            )}
        </div>
    );
}

function EmailSection({
    state,
    setState,
}: {
    state: State;
    setState: React.Dispatch<React.SetStateAction<State>>;
}) {
    return (
        <div className="rounded-md border border-border p-4 space-y-4">
            <div className="flex-x justify-between">
                <span className="flex-x">
                    <Label>Also send an email</Label>
                    <Tooltip
                        content={
                            <>
                                <p>Direct sends only.</p>
                                <p>
                                    Email fires only if the target is cataloged
                                    for email, the recipient's email preference
                                    is on, and they have a primary email
                                    contact.
                                </p>
                            </>
                        }
                    >
                        <IconInfo />
                    </Tooltip>
                </span>

                <Switch
                    checked={state.email_enabled}
                    onCheckedChange={(checked) =>
                        setState((prev) => ({
                            ...prev,
                            email_enabled: checked,
                        }))
                    }
                />
            </div>

            {state.email_enabled && (
                <div className="space-y-4">
                    <WithLabel Label={<Label required>Subject</Label>}>
                        <Input
                            className="w-full!"
                            placeholder="Your weekly digest"
                            value={state.email_subject}
                            onChange={(e) =>
                                setState((prev) => ({
                                    ...prev,
                                    email_subject: e.target.value,
                                }))
                            }
                        />
                    </WithLabel>

                    <WithLabel
                        Label={
                            <span className="flex-x">
                                <Label required>HTML</Label>
                                <Tooltip content="The rendered HTML body. At least one of HTML or Text is required.">
                                    <IconInfo />
                                </Tooltip>
                            </span>
                        }
                    >
                        <Textarea
                            className="w-full! h-40"
                            placeholder="<h1>Hi</h1><p>You have 3 new items.</p>"
                            value={state.email_html}
                            onChange={(e) =>
                                setState((prev) => ({
                                    ...prev,
                                    email_html: e.target.value,
                                }))
                            }
                        />
                    </WithLabel>

                    <WithLabel
                        Label={
                            <span className="flex-x">
                                <Label>Text</Label>
                                <Tooltip content="Plain-text fallback (recommended for deliverability). Auto-derived from HTML when left blank.">
                                    <IconInfo />
                                </Tooltip>
                            </span>
                        }
                    >
                        <Textarea
                            className="w-full! h-24"
                            placeholder="Hi — you have 3 new items."
                            value={state.email_text}
                            onChange={(e) =>
                                setState((prev) => ({
                                    ...prev,
                                    email_text: e.target.value,
                                }))
                            }
                        />
                    </WithLabel>
                </div>
            )}
        </div>
    );
}

// notifyEmailOutcome surfaces the per-medium email delivery outcome so a "sent"
// toast doesn't mask a silently-skipped email (e.g. not_cataloged / no_contact).
function notifyEmailOutcome(res: APIRes<SendNotificationResult>) {
    const email = res?.data?.deliveries?.find((d) => d.medium === "email");

    if (!email) {
        toast.success("Notification sent successfully!");
        return;
    }

    switch (email.status) {
        case "pending":
        case "sent":
            toast.success("Notification sent. Email is on its way.");
            break;
        case "muted":
            toast.warning(
                email.failure_reason === "not_cataloged"
                    ? "Sent in-app. Email skipped — this target isn't cataloged for email."
                    : "Sent in-app. Email skipped — the recipient has email turned off for this target."
            );
            break;
        case "no_contact":
            toast.warning(
                "Sent in-app. Email skipped — the recipient has no primary email contact."
            );
            break;
        case "failed":
            toast.warning(
                `Sent in-app, but email failed${
                    email.failure_reason ? ` (${email.failure_reason})` : ""
                }.`
            );
            break;
        default:
            toast.success("Notification sent successfully!");
    }
}
