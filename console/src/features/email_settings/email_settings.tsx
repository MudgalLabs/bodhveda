import { useEffect, useMemo, useState } from "react";
import {
    Alert,
    Button,
    ErrorMessage,
    IconBadgeInfo,
    IconInfo,
    IconSend,
    Input,
    Label,
    Loading,
    LoadingScreen,
    PageHeading,
    PasswordInput,
    Select,
    toast,
    Tooltip,
    useDocumentTitle,
    WithLabel,
} from "netra";

import { API_BASE_URL } from "@/lib/api";
import { useGetProjectIDFromParams } from "@/features/project/project_hooks";
import {
    useGetEmailSettings,
    useUpsertEmailSettings,
} from "@/features/email_settings/email_settings_hooks";
import {
    EmailProvider,
    ProjectEmailSettings,
} from "@/features/email_settings/email_settings_types";

export function EmailSettings() {
    useDocumentTitle("Email • Bodhveda");

    const id = useGetProjectIDFromParams();

    const { data, isLoading, isFetching, isError } = useGetEmailSettings(id);

    const content = useMemo(() => {
        if (isError) {
            return <ErrorMessage errorMsg="Error loading email settings" />;
        }

        if (isLoading) {
            return <LoadingScreen />;
        }

        if (!data) return null;

        return <EmailSettingsForm settings={data.data} />;
    }, [data, isError, isLoading]);

    return (
        <div>
            <PageHeading>
                <IconSend size={18} />
                <h1>Email</h1>
                {isFetching && <Loading />}
            </PageHeading>

            <p className="text-text-muted paragraph mb-6 max-w-2xl">
                Bring your own email provider. Bodhveda uses these credentials to
                send email on your behalf — your API key is encrypted at rest and
                never shown again after you save it.
            </p>

            {content}
        </div>
    );
}

interface EmailSettingsFormProps {
    settings: ProjectEmailSettings | null;
}

function EmailSettingsForm({ settings }: EmailSettingsFormProps) {
    const id = useGetProjectIDFromParams();
    const configured = settings !== null;

    const [provider, setProvider] = useState<EmailProvider>(
        settings?.provider ?? "resend"
    );
    const [fromName, setFromName] = useState(settings?.from_name ?? "");
    const [fromAddress, setFromAddress] = useState(
        settings?.from_address ?? ""
    );
    const [secret, setSecret] = useState("");
    const [webhookSecret, setWebhookSecret] = useState("");

    const webhookURL = `${API_BASE_URL}/webhooks/email/${id}`;
    const webhookConfigured = settings?.webhook_secret_set ?? false;

    // Keep the form in sync if the underlying settings change (e.g. after a save
    // refetch). The secret inputs always reset to blank — they are write-only.
    useEffect(() => {
        setProvider(settings?.provider ?? "resend");
        setFromName(settings?.from_name ?? "");
        setFromAddress(settings?.from_address ?? "");
        setSecret("");
        setWebhookSecret("");
    }, [settings]);

    const { mutate: upsert, isPending } = useUpsertEmailSettings(id, {
        onSuccess: () => {
            toast.success("Email settings saved");
            setSecret("");
            setWebhookSecret("");
        },
    });

    const handleSubmit = (e: React.FormEvent) => {
        e.preventDefault();

        if (!fromName.trim() || !fromAddress.trim()) return;
        // A secret is required only the first time; afterwards it can be left
        // blank to keep the existing key.
        if (!configured && !secret.trim()) return;

        upsert({
            provider,
            from_name: fromName.trim(),
            from_address: fromAddress.trim(),
            secret: secret.trim() || undefined,
            webhook_secret: webhookSecret.trim() || undefined,
        });
    };

    const disableSave =
        !fromName.trim() ||
        !fromAddress.trim() ||
        (!configured && !secret.trim());

    return (
        <form
            className="border-border-subtle bg-surface-1 flex max-w-2xl flex-col gap-5 rounded-md border p-5"
            onSubmit={handleSubmit}
        >
            {configured && (
                <Alert>
                    <IconBadgeInfo />
                    <p className="text-text-muted">
                        Email is configured with key{" "}
                        <span className="text-text-primary font-medium">
                            {settings?.secret_masked}
                        </span>
                        . Enter a new key below to rotate it, or leave it blank to
                        keep the current one.
                    </p>
                </Alert>
            )}

            <WithLabel Label={<Label>Provider</Label>}>
                <Select
                    classNames={{ trigger: "w-full!" }}
                    options={[{ label: "Resend", value: "resend" }]}
                    value={provider}
                    onValueChange={(v) => setProvider(v as EmailProvider)}
                    required
                />
            </WithLabel>

            <WithLabel
                Label={
                    <Label className="flex-x">
                        API Key
                        <Tooltip
                            content={
                                <p>
                                    Your provider's secret API key (Resend keys
                                    start with <strong>re_</strong>). It is
                                    encrypted at rest and never shown again.
                                </p>
                            }
                        >
                            <IconInfo />
                        </Tooltip>
                    </Label>
                }
            >
                <PasswordInput
                    className="w-full!"
                    placeholder={
                        configured
                            ? "Leave blank to keep the current key"
                            : "re_..."
                    }
                    value={secret}
                    onChange={(e) => setSecret(e.target.value)}
                    autoComplete="off"
                />
            </WithLabel>

            <WithLabel
                Label={
                    <Label className="flex-x">
                        From name
                        <Tooltip content="The sender name recipients see, e.g. your product name.">
                            <IconInfo />
                        </Tooltip>
                    </Label>
                }
            >
                <Input
                    className="w-full!"
                    placeholder="Acme"
                    type="text"
                    required
                    maxLength={128}
                    value={fromName}
                    onChange={(e) => setFromName(e.target.value)}
                />
            </WithLabel>

            <WithLabel
                Label={
                    <Label className="flex-x">
                        From address
                        <Tooltip content="A verified sending address on your provider, e.g. hey@acme.com.">
                            <IconInfo />
                        </Tooltip>
                    </Label>
                }
            >
                <Input
                    className="w-full!"
                    placeholder="hey@acme.com"
                    type="email"
                    required
                    maxLength={255}
                    value={fromAddress}
                    onChange={(e) => setFromAddress(e.target.value)}
                />
            </WithLabel>

            <div className="border-border-subtle border-t pt-5">
                <h2 className="text-text-primary mb-1 font-medium">
                    Delivery status webhook
                </h2>
                <p className="text-text-muted paragraph mb-4">
                    Add this URL as a webhook endpoint in your Resend dashboard
                    to receive delivered / bounced / complained / opened events.
                    Resend generates a signing secret — paste it below so
                    Bodhveda can verify the events.
                </p>

                <WithLabel Label={<Label>Webhook URL</Label>}>
                    <Input
                        className="w-full!"
                        type="text"
                        readOnly
                        value={webhookURL}
                        onFocus={(e) => e.currentTarget.select()}
                    />
                </WithLabel>

                <div className="mt-5">
                    {webhookConfigured && (
                        <Alert className="mb-4">
                            <IconBadgeInfo />
                            <p className="text-text-muted">
                                Webhook signing secret is configured (
                                <span className="text-text-primary font-medium">
                                    {settings?.webhook_secret_masked}
                                </span>
                                ). Enter a new secret to rotate it, or leave it
                                blank to keep the current one.
                            </p>
                        </Alert>
                    )}

                    <WithLabel
                        Label={
                            <Label className="flex-x">
                                Webhook signing secret
                                <Tooltip
                                    content={
                                        <p>
                                            The <strong>Signing Secret</strong>{" "}
                                            Resend shows when you create the
                                            webhook endpoint (starts with{" "}
                                            <strong>whsec_</strong>). Distinct
                                            from your API key. Encrypted at rest
                                            and never shown again.
                                        </p>
                                    }
                                >
                                    <IconInfo />
                                </Tooltip>
                            </Label>
                        }
                    >
                        <PasswordInput
                            className="w-full!"
                            placeholder={
                                webhookConfigured
                                    ? "Leave blank to keep the current secret"
                                    : "whsec_..."
                            }
                            value={webhookSecret}
                            onChange={(e) => setWebhookSecret(e.target.value)}
                            autoComplete="off"
                        />
                    </WithLabel>
                </div>
            </div>

            <div className="flex justify-end">
                <Tooltip
                    content="Some required fields are missing"
                    disabled={!disableSave}
                >
                    <Button
                        type="submit"
                        disabled={disableSave}
                        loading={isPending}
                    >
                        Save
                    </Button>
                </Tooltip>
            </div>
        </form>
    );
}
