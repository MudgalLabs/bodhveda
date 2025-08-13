import { FC, ReactNode, useEffect, useState } from "react";
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
    IconInfo,
    Input,
    Label,
    PasswordInput,
    Select,
    toast,
    Tooltip,
    WithLabel,
} from "netra";
import { APIKeyScope } from "@/features/api_key/api_key_types";
import { useCreateAPIKey } from "@/features/api_key/api_key_hooks";

interface CreateprojectModalProps {
    renderTrigger: () => ReactNode;
}

export const CreateAPIKeyModal: FC<CreateprojectModalProps> = ({
    renderTrigger,
}) => {
    const projectID = useGetProjectIDFromParams();

    const [open, setOpen] = useState(false);
    const [name, setName] = useState("");
    const [scope, setScope] = useState<APIKeyScope>("recipient");

    const [token, setToken] = useState("");

    const { mutate: create, isPending } = useCreateAPIKey({
        onSuccess: (res) => {
            toast.success(`API Key ${name} created successfully`);
            const token = res.data.data;
            setToken(token);
        },
    });

    const handleSubmit = (e: React.FormEvent) => {
        e.preventDefault();

        if (!name.trim()) {
            return;
        }

        create({
            projectID,
            payload: {
                name,
                scope,
            },
        });
    };

    const disableCreate = !name.trim();

    useEffect(() => {
        if (open) {
            setName("");
            setScope("recipient");
            setToken("");
        }
    }, [open]);

    return (
        <Dialog open={open} onOpenChange={setOpen}>
            <DialogTrigger asChild>{renderTrigger()}</DialogTrigger>

            <DialogContent>
                <DialogHeader>
                    <DialogTitle>
                        {token ? "View" : "Create"} API Key
                    </DialogTitle>
                </DialogHeader>

                {token ? (
                    <div className="space-y-4">
                        <Alert>
                            <IconBadgeInfo />
                            <p className="text-text-muted">
                                You can only see this key once.{" "}
                                <span className="text-text-primary font-medium">
                                    Store it safely
                                </span>
                                .
                            </p>
                        </Alert>

                        <PasswordInput className="w-full!" value={token} />

                        <DialogFooter>
                            <Button
                                className="ml-auto"
                                onClick={() => setOpen(false)}
                            >
                                Done
                            </Button>
                        </DialogFooter>
                    </div>
                ) : (
                    <form
                        className="flex flex-col gap-4"
                        onSubmit={handleSubmit}
                    >
                        <WithLabel Label={<Label>Name</Label>}>
                            <Input
                                className="w-full!"
                                placeholder="Production"
                                type="text"
                                required
                                maxLength={64}
                                value={name}
                                onChange={(e) => setName(e.target.value)}
                            />
                        </WithLabel>

                        <WithLabel
                            Label={
                                <Label className="flex-x">
                                    Scope
                                    <Tooltip
                                        content={
                                            <div className="space-y-2">
                                                <p>
                                                    <strong>
                                                        Full access:
                                                    </strong>{" "}
                                                    Allows to create, read,
                                                    update, and delete{" "}
                                                    <strong>all</strong>{" "}
                                                    resources.
                                                    <br />
                                                    Only this scope can send
                                                    notifications.
                                                </p>

                                                <p>
                                                    <strong>
                                                        Recipient access:
                                                    </strong>{" "}
                                                    Allows to perform all
                                                    recipient operations, like
                                                    fetch notifications, mark a
                                                    notification as read, delete
                                                    a notification, preferences,
                                                    mutes, subs, and more.
                                                </p>
                                            </div>
                                        }
                                    >
                                        <IconInfo />
                                    </Tooltip>
                                </Label>
                            }
                        >
                            <Select
                                classNames={{
                                    trigger: "w-full!",
                                }}
                                options={[
                                    {
                                        label: "Full access",
                                        value: "full",
                                    },
                                    {
                                        label: "Recipient access",
                                        value: "recipient",
                                    },
                                ]}
                                value={scope}
                                onValueChange={(v) =>
                                    setScope(v as APIKeyScope)
                                }
                                required
                            />
                        </WithLabel>

                        <DialogFooter>
                            <Tooltip
                                content="Some required fields are missing"
                                disabled={!disableCreate}
                            >
                                <Button
                                    type="submit"
                                    disabled={disableCreate}
                                    loading={isPending}
                                >
                                    Create
                                </Button>
                            </Tooltip>
                        </DialogFooter>
                    </form>
                )}
            </DialogContent>
        </Dialog>
    );
};
