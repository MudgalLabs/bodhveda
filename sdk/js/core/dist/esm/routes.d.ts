export declare const ROUTES: {
    notifications: {
        send: string;
    };
    recipients: {
        create: string;
        createBatch: string;
        get: (recipientID: string) => string;
        update: (recipientID: string) => string;
        delete: (recipientID: string) => string;
        notifications: {
            list: (recipientID: string) => string;
            unreadCount: (recipientID: string) => string;
            udpateState: (recipientID: string) => string;
            delete: (recipientID: string) => string;
        };
        preferences: {
            list: (recipientID: string) => string;
            set: (recipientID: string) => string;
            check: (recipientID: string) => string;
        };
    };
};
//# sourceMappingURL=routes.d.ts.map