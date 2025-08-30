export const ROUTES = {
    notifications: {
        send: "/notifications/send",
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
    },
};
