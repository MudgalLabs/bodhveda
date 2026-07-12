import { FC, useState } from "react";
import {
    Button,
    Dialog,
    DialogContent,
    DialogHeader,
    DialogTitle,
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
import { useGetProjectIDFromParams } from "@/features/project/project_hooks";
import {
    useCreateRecipientContact,
    useDeleteRecipientContact,
    useGetRecipientContacts,
    useUpdateRecipientContact,
} from "@/features/recipient/contact_hooks";
import {
    Medium,
    RecipientContact,
} from "@/features/recipient/contact_types";
import { apiErrorHandler } from "@/lib/api";

const MEDIUM_OPTIONS: { label: string; value: Medium }[] = [
    { label: "Email", value: "email" },
    { label: "SMS", value: "sms" },
    { label: "Web Push", value: "web_push" },
    { label: "Mobile Push", value: "mobile_push" },
];

const MEDIUM_LABEL: Record<Medium, string> = {
    email: "Email",
    sms: "SMS",
    web_push: "Web Push",
    mobile_push: "Mobile Push",
};

interface RecipientContactsModalProps {
    recipientID: string;
    open: boolean;
    setOpen: (open: boolean) => void;
}

export const RecipientContactsModal: FC<RecipientContactsModalProps> = ({
    recipientID,
    open,
    setOpen,
}) => {
    const projectID = useGetProjectIDFromParams();

    const { data, isLoading, isError, isFetching } = useGetRecipientContacts(
        projectID,
        recipientID,
        open
    );

    const contacts = data?.data?.contacts ?? [];

    return (
        <Dialog open={open} onOpenChange={setOpen}>
            <DialogContent>
                <DialogHeader>
                    <DialogTitle className="flex-x">
                        Contacts
                        {isFetching && <Loading />}
                    </DialogTitle>
                </DialogHeader>

                <p className="text-foreground-muted text-sm">
                    Contact addresses for{" "}
                    <span className="select-text!">{recipientID}</span>. Only
                    email is used today; other mediums are reserved for future
                    delivery transports.
                </p>

                <div className="space-y-3">
                    {isError && (
                        <ErrorMessage errorMsg="Error loading contacts" />
                    )}

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

                <AddContactForm
                    projectID={projectID}
                    recipientID={recipientID}
                />
            </DialogContent>
        </Dialog>
    );
};

interface ContactRowProps {
    projectID: string;
    recipientID: string;
    contact: RecipientContact;
}

function ContactRow({ projectID, recipientID, contact }: ContactRowProps) {
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
            onSuccess: () => toast.success("Contact deleted"),
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
                    onClick={() => remove({ contactID: contact.id })}
                >
                    <IconTrash size={16} />
                </Button>
            </div>
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
