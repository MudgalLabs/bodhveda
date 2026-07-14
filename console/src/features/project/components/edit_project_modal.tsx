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
    Tooltip,
    WithLabel,
} from "netra";

import { useUpdateProject } from "@/features/project/project_hooks";

interface EditProjectModalProps {
    open: boolean;
    setOpen: (open: boolean) => void;
    id: number;
    name: string;
}

export function EditProjectModal(props: EditProjectModalProps) {
    const { open, setOpen, id, name } = props;

    const [newName, setNewName] = useState(name);

    const { mutate: update, isPending } = useUpdateProject({
        onSuccess: () => {
            toast.success(`Project renamed to ${newName.trim()}`);
            setOpen(false);
        },
    });

    useEffect(() => {
        if (open) {
            setNewName(name);
        }
    }, [open, name]);

    const trimmed = newName.trim();
    const disableSave = !trimmed || trimmed === name;

    const handleSubmit = (e: React.FormEvent) => {
        e.preventDefault();

        if (disableSave) {
            return;
        }

        update({ id, name: trimmed });
    };

    return (
        <Dialog open={open} onOpenChange={setOpen}>
            <DialogContent>
                <DialogHeader>
                    <DialogTitle>Edit Project</DialogTitle>
                </DialogHeader>

                <form className="flex flex-col gap-4" onSubmit={handleSubmit}>
                    <WithLabel Label={<Label>Name</Label>}>
                        <Input
                            className="w-full!"
                            placeholder="Swing Trading Account"
                            type="text"
                            required
                            maxLength={64}
                            value={newName}
                            onChange={(e) => setNewName(e.target.value)}
                        />
                    </WithLabel>

                    <DialogFooter>
                        <Button
                            variant="secondary"
                            type="button"
                            onClick={() => setOpen(false)}
                        >
                            Cancel
                        </Button>

                        <Tooltip
                            content="Enter a new name to save"
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
                    </DialogFooter>
                </form>
            </DialogContent>
        </Dialog>
    );
}
