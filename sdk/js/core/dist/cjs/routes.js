"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.ROUTES = void 0;
exports.ROUTES = {
    notifications: {
        send: "/notifications/send",
    },
    recipients: {
        create: "/recipients",
        createBatch: "/recipients/batch",
        get: (recipientID) => `/recipients/${recipientID}`,
        update: (recipientID) => `/recipients/${recipientID}`,
        delete: (recipientID) => `/recipients/${recipientID}`,
        notifications: {
            list: (recipientID) => `/recipients/${recipientID}/notifications`,
            unreadCount: (recipientID) => `/recipients/${recipientID}/notifications/unread-count`,
            udpateState: (recipientID) => `/recipients/${recipientID}/notifications`,
            delete: (recipientID) => `/recipients/${recipientID}/notifications`,
        },
        preferences: {
            list: (recipientID) => `/recipients/${recipientID}/preferences`,
            set: (recipientID) => `/recipients/${recipientID}/preferences`,
            check: (recipientID) => `/recipients/${recipientID}/preferences/check`,
        },
    },
};
//# sourceMappingURL=routes.js.map