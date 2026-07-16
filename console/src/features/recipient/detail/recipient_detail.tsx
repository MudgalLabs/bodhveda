import { useMemo } from "react";
import { Link } from "@tanstack/react-router";
import {
    buttonVariants,
    ErrorMessage,
    formatDate,
    IconArrowLeft,
    IconMegaphone,
    IconTarget,
    IconUsers,
    Loading,
    LoadingScreen,
    PageHeading,
    Separator,
    Tabs,
    TabsContent,
    TabsList,
    TabsTrigger,
    Tooltip,
    useDocumentTitle,
} from "netra";

import { useGetProjectIDFromParams } from "@/features/project/project_hooks";
import { useGetRecipient } from "@/features/recipient/recipient_hooks";
import { RecipientContactsPanel } from "@/features/recipient/detail/recipient_contacts_panel";
import { RecipientNotificationsPanel } from "@/features/recipient/detail/recipient_notifications_panel";
import { RecipientPreferencesPanel } from "@/features/recipient/detail/recipient_preferences_panel";
import { RecipientListItem } from "@/features/recipient/recipient_types";

interface RecipientDetailProps {
    recipientID: string;
}

export function RecipientDetail({ recipientID }: RecipientDetailProps) {
    useDocumentTitle(`${recipientID} • Recipients • Bodhveda`);

    const projectID = useGetProjectIDFromParams();
    const { data, isLoading, isError, isFetching } = useGetRecipient(
        projectID,
        recipientID
    );

    const recipient = data?.data;

    const content = useMemo(() => {
        if (isError) {
            return (
                <ErrorMessage errorMsg={`Could not load "${recipientID}"`} />
            );
        }

        if (isLoading) {
            return <LoadingScreen />;
        }

        if (!recipient) {
            return <ErrorMessage errorMsg={`Recipient "${recipientID}" not found`} />;
        }

        return (
            <Tabs defaultValue="overview">
                <TabsList>
                    <TabsTrigger value="overview">Overview</TabsTrigger>
                    <TabsTrigger value="notifications">
                        Notifications
                    </TabsTrigger>
                    <TabsTrigger value="preferences">Preferences</TabsTrigger>
                    <TabsTrigger value="contacts">Contacts</TabsTrigger>
                </TabsList>

                <TabsContent value="overview" className="pt-4">
                    <OverviewTab recipient={recipient} />
                </TabsContent>

                <TabsContent value="notifications" className="pt-4">
                    <RecipientNotificationsPanel
                        projectID={projectID}
                        recipientID={recipientID}
                    />
                </TabsContent>

                <TabsContent value="preferences" className="pt-4">
                    <RecipientPreferencesPanel
                        projectID={projectID}
                        recipientID={recipientID}
                    />
                </TabsContent>

                <TabsContent value="contacts" className="pt-4">
                    <RecipientContactsPanel
                        projectID={projectID}
                        recipientID={recipientID}
                    />
                </TabsContent>
            </Tabs>
        );
    }, [isError, isLoading, recipient, recipientID, projectID]);

    return (
        <div>
            <div className="mb-2">
                <Link
                    to="/projects/$id/recipients"
                    params={{ id: projectID }}
                    className={buttonVariants({
                        variant: "ghost",
                        size: "small",
                    })}
                >
                    <IconArrowLeft size={16} />
                    Recipients
                </Link>
            </div>

            <PageHeading>
                <IconUsers size={18} />
                <h1 className="select-text!">{recipientID}</h1>
                {isFetching && <Loading />}
            </PageHeading>

            {content}
        </div>
    );
}

function OverviewTab({ recipient }: { recipient: RecipientListItem }) {
    return (
        <div className="max-w-2xl space-y-4">
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <Field label="Recipient ID">
                    <span className="select-text!">{recipient.id}</span>
                </Field>

                <Field label="Name">
                    {recipient.name || (
                        <span className="text-foreground-muted">—</span>
                    )}
                </Field>

                <Field label="Created">
                    {formatDate(new Date(recipient.created_at), { time: true })}
                </Field>
            </div>

            <Separator />

            <div>
                <p className="text-foreground-muted text-xs mb-2">
                    Notifications
                </p>
                <div className="flex-x gap-x-6!">
                    <Tooltip content="Direct notifications sent to this recipient">
                        <span className="flex-x">
                            <IconTarget size={16} />
                            {recipient.direct_notifications_count} direct
                        </span>
                    </Tooltip>
                    <Tooltip content="Broadcast notifications this recipient received">
                        <span className="flex-x">
                            <IconMegaphone size={16} />
                            {recipient.broadcast_notifications_count} broadcast
                        </span>
                    </Tooltip>
                </div>
            </div>
        </div>
    );
}

function Field({
    label,
    children,
}: {
    label: string;
    children: React.ReactNode;
}) {
    return (
        <div>
            <p className="text-foreground-muted text-xs mb-1">{label}</p>
            <div className="text-sm">{children}</div>
        </div>
    );
}

