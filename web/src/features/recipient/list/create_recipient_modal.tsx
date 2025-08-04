import { FC, ReactNode, useEffect, useState } from "react";
import { useGetProjectIDFromParams } from "@/features/project/project_hooks";
import {
    Button,
    Dialog,
    DialogContent,
    DialogFooter,
    DialogHeader,
    DialogTitle,
    DialogTrigger,
    Input,
    Label,
    toast,
    WithLabel,
} from "netra";
import { useCreateRecipient } from "@/features/recipient/recipient_hooks";
import { apiErrorHandler } from "@/lib/api";

interface CreateRecipientModalProps {
    renderTrigger: () => ReactNode;
}

export const CreateRecipientModal: FC<CreateRecipientModalProps> = ({
    renderTrigger,
}) => {
    const projectID = useGetProjectIDFromParams();

    const [open, setOpen] = useState(false);
    const [recipientID, setRecipientID] = useState("");
    const [name, setName] = useState("");

    const { mutate: create, isPending } = useCreateRecipient({
        onSuccess: () => {
            toast.success(`Recipient ${recipientID} created successfully`);
            setOpen(false);
        },
        onError: apiErrorHandler,
    });

    const handleSubmit = (e: React.FormEvent) => {
        e.preventDefault();

        if (!recipientID.trim()) return;

        create({
            projectID,
            payload: {
                recipient_id: recipientID,
                name: name.trim() || null,
            },
        });
    };

    const disableCreate = !recipientID.trim();

    useEffect(() => {
        if (open) {
            setRecipientID("");
            setName("");
        }
    }, [open]);

    return (
        <Dialog open={open} onOpenChange={setOpen}>
            <DialogTrigger asChild>{renderTrigger()}</DialogTrigger>
            <DialogContent>
                <DialogHeader>
                    <DialogTitle>Create Recipient</DialogTitle>
                </DialogHeader>
                <form className="flex flex-col gap-4" onSubmit={handleSubmit}>
                    <WithLabel Label={<Label>Recipient ID</Label>}>
                        <Input
                            className="w-full!"
                            placeholder="user@example.com OR 12345"
                            required
                            maxLength={256}
                            value={recipientID}
                            onChange={(e) => setRecipientID(e.target.value)}
                        />
                    </WithLabel>
                    <WithLabel Label={<Label>Name</Label>}>
                        <Input
                            className="w-full!"
                            placeholder="John Doe"
                            type="text"
                            maxLength={256}
                            value={name}
                            onChange={(e) => setName(e.target.value)}
                        />
                    </WithLabel>
                    <DialogFooter>
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
