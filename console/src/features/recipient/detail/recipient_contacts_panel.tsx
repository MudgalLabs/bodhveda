import { useState } from "react";
import {
    Button,
    ErrorMessage,
    IconBadgeCheck,
    IconPlus,
    IconTrash,
    Input,
    Label,
    Loading,
    Select,
    Separator,
    Tag,
    toast,
    Tooltip,
    WithLabel,
} from "netra";
import {
    useCreateRecipientContact,
    useDeleteRecipientContact,
    useGetRecipientContacts,
    useUpdateRecipientContact,
} from "@/features/recipient/contact_hooks";
import { Medium, RecipientContact } from "@/features/recipient/contact_types";
import { ConfirmDialog } from "@/components/confirm_dialog";
import { apiErrorHandler } from "@/lib/api";

// Only the mediums that actually deliver today — mirrors `Medium.Active()` in
// `api/internal/model/enum/medium.go`. The API accepts the scaffolded contact
// mediums (sms, web_push, mobile_push), but offering them here would let someone
// store an address no transport can ever reach.
const MEDIUM_OPTIONS: { label: string; value: Medium }[] = [
    { label: "Email", value: "email" },
];

// Every medium, not just the offered ones: a contact created through the API can
// carry any of them, and the rows above still have to render it.
const MEDIUM_LABEL: Record<Medium, string> = {
    email: "Email",
    sms: "SMS",
    web_push: "Web Push",
    mobile_push: "Mobile Push",
};

interface RecipientContactsPanelProps {
    projectID: string;
    recipientID: string;
}

/**
 * The recipient's contact addresses.
 *
 * This was a modal hung off the recipient list's row actions (Phase 1) purely
 * because no recipient detail page existed to host it. It does now, so this is
 * a plain panel.
 */
export function RecipientContactsPanel({
    projectID,
    recipientID,
}: RecipientContactsPanelProps) {
    const { data, isLoading, isError } = useGetRecipientContacts(
        projectID,
        recipientID
    );

    const contacts = data?.data?.contacts ?? [];

    return (
        <div className="space-y-4 max-w-2xl">
            <p className="text-foreground-muted text-sm">
                Where this recipient can be reached. Email delivery requires a{" "}
                <strong>primary</strong> email contact — without one, an email
                send records a <code>no_contact</code> outcome. Email is the only
                medium that delivers today.
            </p>

            <div className="space-y-3">
                {isError && <ErrorMessage errorMsg="Error loading contacts" />}

                {isLoading && <Loading />}

                {!isLoading && !isError && contacts.length === 0 && (
                    <p className="text-foreground-muted text-sm">
                        No contacts yet.
                    </p>
                )}

                {contacts.map((contact) => (
                    <ContactRow
                        key={contact.id}
                        projectID={projectID}
                        recipientID={recipientID}
                        contact={contact}
                    />
                ))}
            </div>

            <Separator />

            <AddContactForm projectID={projectID} recipientID={recipientID} />
        </div>
    );
}

interface ContactRowProps {
    projectID: string;
    recipientID: string;
    contact: RecipientContact;
}

function ContactRow({ projectID, recipientID, contact }: ContactRowProps) {
    const [confirmOpen, setConfirmOpen] = useState(false);

    const { mutate: update, isPending: isUpdating } = useUpdateRecipientContact(
        projectID,
        recipientID,
        {
            onSuccess: () => toast.success("Contact updated"),
            onError: apiErrorHandler,
        }
    );

    const { mutate: remove, isPending: isDeleting } = useDeleteRecipientContact(
        projectID,
        recipientID,
        {
            onSuccess: () => {
                toast.success("Contact deleted");
                setConfirmOpen(false);
            },
            onError: apiErrorHandler,
        }
    );

    return (
        <div className="flex items-center justify-between gap-2 border border-border rounded-md p-2">
            <div className="min-w-0 space-y-1">
                <div className="flex-x flex-wrap gap-1!">
                    <span className="select-text! truncate">
                        {contact.address}
                    </span>
                    {contact.is_primary && (
                        <Tag variant="success" size="small">
                            Primary
                        </Tag>
                    )}
                    {contact.verified_at ? (
                        <Tag variant="success" size="small">
                            <IconBadgeCheck size={12} /> Verified
                        </Tag>
                    ) : (
                        <Tag variant="muted" size="small">
                            Unverified
                        </Tag>
                    )}
                </div>
                <span className="text-foreground-muted text-xs">
                    {MEDIUM_LABEL[contact.medium] ?? contact.medium}
                </span>
            </div>

            <div className="flex-x">
                {!contact.is_primary && (
                    <Tooltip content="Set as primary">
                        <Button
                            variant="ghost"
                            size="small"
                            loading={isUpdating}
                            onClick={() =>
                                update({
                                    contactID: contact.id,
                                    payload: { is_primary: true },
                                })
                            }
                        >
                            Make primary
                        </Button>
                    </Tooltip>
                )}
                <Button
                    variant="destructive"
                    size="icon"
                    loading={isDeleting}
                    onClick={() => setConfirmOpen(true)}
                >
                    <IconTrash size={16} />
                </Button>
            </div>

            <ConfirmDialog
                open={confirmOpen}
                onOpenChange={setConfirmOpen}
                title="Delete contact"
                description={
                    <p>
                        Delete{" "}
                        <span className="font-bold text-text-primary">
                            {contact.address}
                        </span>
                        ? This recipient can no longer be reached at this
                        address.
                    </p>
                }
                confirmLabel="Delete contact"
                loading={isDeleting}
                onConfirm={() => remove({ contactID: contact.id })}
            />
        </div>
    );
}

interface AddContactFormProps {
    projectID: string;
    recipientID: string;
}

function AddContactForm({ projectID, recipientID }: AddContactFormProps) {
    const [medium, setMedium] = useState<Medium>("email");
    const [address, setAddress] = useState("");
    const [isPrimary, setIsPrimary] = useState(true);

    const { mutate: create, isPending } = useCreateRecipientContact(
        projectID,
        recipientID,
        {
            onSuccess: () => {
                toast.success("Contact added");
                setAddress("");
            },
            onError: apiErrorHandler,
        }
    );

    const handleSubmit = (e: React.FormEvent) => {
        e.preventDefault();
        if (!address.trim()) return;
        create({
            payload: {
                medium,
                address: address.trim(),
                is_primary: isPrimary,
            },
        });
    };

    return (
        <form className="flex flex-col gap-3" onSubmit={handleSubmit}>
            <WithLabel Label={<Label>Medium</Label>}>
                <Select
                    classNames={{ trigger: "w-full!" }}
                    options={MEDIUM_OPTIONS}
                    value={medium}
                    onValueChange={(v) => setMedium(v as Medium)}
                    required
                />
            </WithLabel>

            <WithLabel Label={<Label>Address</Label>}>
                <Input
                    className="w-full!"
                    placeholder={
                        medium === "email"
                            ? "user@example.com"
                            : "Contact address / token"
                    }
                    required
                    maxLength={512}
                    value={address}
                    onChange={(e) => setAddress(e.target.value)}
                />
            </WithLabel>

            <label className="flex-x cursor-pointer text-sm select-none">
                <input
                    type="checkbox"
                    checked={isPrimary}
                    onChange={(e) => setIsPrimary(e.target.checked)}
                />
                Set as primary for this medium
            </label>

            <div className="flex justify-end">
                <Button
                    type="submit"
                    disabled={!address.trim()}
                    loading={isPending}
                >
                    <IconPlus size={16} />
                    Add contact
                </Button>
            </div>
        </form>
    );
}
