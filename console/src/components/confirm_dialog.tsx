import {
    Button,
    Dialog,
    DialogContent,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "netra";

interface ConfirmDialogProps {
    open: boolean;
    onOpenChange: (open: boolean) => void;
    title: string;
    /** The body. A string renders as a paragraph; pass nodes for richer copy. */
    description: React.ReactNode;
    confirmLabel: string;
    onConfirm: () => void;
    loading?: boolean;
    /** The confirm button's intent — destructive by default, since that is why
     * a confirm exists at all. */
    variant?: "destructive" | "default";
}

/**
 * A yes/no confirmation for an action that is easy to trigger by mistake — a
 * trash icon, a row's Delete. It is deliberately lighter than the type-DELETE
 * modals (API key, project, preference): those guard irreversible, high-blast
 * deletes and want friction; this one only wants a beat between the click and
 * the consequence. Reach for the type-DELETE modal instead when the target is
 * something a customer would be alarmed to lose silently.
 */
export function ConfirmDialog({
    open,
    onOpenChange,
    title,
    description,
    confirmLabel,
    onConfirm,
    loading = false,
    variant = "destructive",
}: ConfirmDialogProps) {
    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent>
                <DialogHeader>
                    <DialogTitle>{title}</DialogTitle>
                    {typeof description === "string" ? (
                        <p>{description}</p>
                    ) : (
                        description
                    )}
                </DialogHeader>

                <DialogFooter>
                    <Button
                        type="button"
                        variant="secondary"
                        onClick={() => onOpenChange(false)}
                    >
                        Cancel
                    </Button>
                    <Button
                        type="button"
                        variant={
                            variant === "destructive" ? "destructive" : "primary"
                        }
                        loading={loading}
                        onClick={onConfirm}
                    >
                        {confirmLabel}
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    );
}
