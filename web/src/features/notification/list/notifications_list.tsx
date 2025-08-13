import { Button, IconSend, PageHeading } from "netra";

import { useSidebar } from "@/components/sidebar/sidebar";
import { useGetProjectIDFromParams } from "@/features/project/project_hooks";
import { useState } from "react";
import { NotificationKind } from "../notification_types";
import { SendNotificationModal } from "../components/send_notification_modal";
import { NotificationKindToggle } from "../components/notification_kind_toggle";

export function NotificationList() {
    const { isOpen, toggleSidebar } = useSidebar();
    const id = useGetProjectIDFromParams();

    const [kind, setKind] = useState<NotificationKind>("direct");

    return (
        <div>
            <PageHeading
                heading="Notifications"
                // loading={isLoading}
                isOpen={isOpen}
                toggleSidebar={toggleSidebar}
            />

            <div className="flex justify-between mb-4">
                <NotificationKindToggle kind={kind} setKind={setKind} />

                <SendNotificationModal
                    renderTrigger={() => (
                        <Button>
                            <IconSend size={16} />
                            Send Notification
                        </Button>
                    )}
                />
            </div>
        </div>
    );
}
