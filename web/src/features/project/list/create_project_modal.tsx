import { FC, ReactNode, useEffect, useState } from "react";
import { useCreateProject } from "@/features/project/project_hooks";
import {
    Button,
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
    DialogTrigger,
    Input,
    Label,
    toast,
    Tooltip,
    WithLabel,
} from "netra";

interface CreateprojectModalProps {
    renderTrigger: () => ReactNode;
}

export const CreateProjectModal: FC<CreateprojectModalProps> = ({
    renderTrigger,
}) => {
    const [open, setOpen] = useState(false);
    const [name, setName] = useState("");

    const { mutate: create, isPending } = useCreateProject({
        onSuccess: () => {
            toast.success(`Project ${name} created successfully`);
            setOpen(false);
        },
    });

    const handleSubmit = (e: React.FormEvent) => {
        e.preventDefault();

        if (!name.trim()) {
            return;
        }

        create({
            name,
        });
    };

    const disableCreate = !name.trim();

    useEffect(() => {
        if (open) {
            setName("");
        }
    }, [open]);

    return (
        <Dialog open={open} onOpenChange={setOpen}>
            <DialogTrigger asChild>{renderTrigger()}</DialogTrigger>

            <DialogContent>
                <DialogHeader>
                    <DialogTitle>Create Project</DialogTitle>
                    <DialogDescription className="max-w-[80%]">
                        Create a project for your app to start sending
                        notifications.
                    </DialogDescription>
                </DialogHeader>

                <form className="flex flex-col gap-4" onSubmit={handleSubmit}>
                    <WithLabel Label={<Label>Name</Label>}>
                        <Input
                            className="w-full!"
                            placeholder="Swing Trading Account"
                            type="text"
                            required
                            maxLength={64}
                            value={name}
                            onChange={(e) => setName(e.target.value)}
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
            </DialogContent>
        </Dialog>
    );
};
