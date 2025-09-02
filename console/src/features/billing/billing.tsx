import { IconCreditCard, PageHeading, useDocumentTitle } from "netra";

export function Billing() {
    useDocumentTitle("Billing  • Bodhveda");

    return (
        <PageHeading>
            <IconCreditCard size={18} />
            <h1>Billing</h1>
        </PageHeading>
    );
}
