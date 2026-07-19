export const ROUTES = {
    notifications: {
        send: "/notifications/send",
        get: (notificationID: number) => `/notifications/${notificationID}`,
    },

    // Project-scoped preference CATALOG (project comes from the API key). Distinct
    // from recipients.preferences, which is a single recipient's own toggles.
    preferences: {
        list: "/preferences",
        create: "/preferences",
        upsertMany: "/preferences",
        get: (preferenceID: number) => `/preferences/${preferenceID}`,
        update: (preferenceID: number) => `/preferences/${preferenceID}`,
        delete: (preferenceID: number) => `/preferences/${preferenceID}`,
    },

    recipients: {
        create: "/recipients",
        createBatch: "/recipients/batch",
        get: (recipientID: string) =>
            `/recipients/${encodeURIComponent(recipientID)}`,
        update: (recipientID: string) =>
            `/recipients/${encodeURIComponent(recipientID)}`,
        delete: (recipientID: string) =>
            `/recipients/${encodeURIComponent(recipientID)}`,

        notifications: {
            list: (recipientID: string) =>
                `/recipients/${encodeURIComponent(recipientID)}/notifications`,
            unreadCount: (recipientID: string) =>
                `/recipients/${encodeURIComponent(
                    recipientID
                )}/notifications/unread-count`,
            udpateState: (recipientID: string) =>
                `/recipients/${encodeURIComponent(recipientID)}/notifications`,
            delete: (recipientID: string) =>
                `/recipients/${encodeURIComponent(recipientID)}/notifications`,
        },

        preferences: {
            list: (recipientID: string) =>
                `/recipients/${encodeURIComponent(recipientID)}/preferences`,
            set: (recipientID: string) =>
                `/recipients/${encodeURIComponent(recipientID)}/preferences`,
            check: (recipientID: string) =>
                `/recipients/${encodeURIComponent(
                    recipientID
                )}/preferences/check`,
        },

        contacts: {
            list: (recipientID: string) =>
                `/recipients/${encodeURIComponent(recipientID)}/contacts`,
            create: (recipientID: string) =>
                `/recipients/${encodeURIComponent(recipientID)}/contacts`,
            setPrimary: (recipientID: string) =>
                `/recipients/${encodeURIComponent(recipientID)}/contacts`,
            update: (recipientID: string, contactID: number) =>
                `/recipients/${encodeURIComponent(
                    recipientID
                )}/contacts/${contactID}`,
            delete: (recipientID: string, contactID: number) =>
                `/recipients/${encodeURIComponent(
                    recipientID
                )}/contacts/${contactID}`,
        },
    },
};
