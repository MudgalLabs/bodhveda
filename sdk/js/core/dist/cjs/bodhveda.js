"use strict";
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
exports.Bodhveda = void 0;
const axios_1 = __importDefault(require("axios"));
const routes_1 = require("./routes");
class Bodhveda {
    constructor(apiKey, options = {}) {
        const { apiURL = "https://api.bodhveda.com" } = options;
        const client = axios_1.default.create({
            baseURL: apiURL,
            headers: {
                "Content-Type": "application/json",
                Authorization: `Bearer ${apiKey}`,
            },
        });
        client.interceptors.response.use((response) => response.data, // Unwrap the main data object from the response.
        (error) => {
            var _a, _b;
            if (axios_1.default.isAxiosError(error) && ((_a = error.response) === null || _a === void 0 ? void 0 : _a.data)) {
                throw (_b = error.response) === null || _b === void 0 ? void 0 : _b.data; // Throw the API's error response directly.
            }
            else {
                throw error;
            }
        });
        this.notifications = new Notifications(client);
        this.recipients = new Recipients(client);
    }
}
exports.Bodhveda = Bodhveda;
class Notifications {
    constructor(client) {
        this.client = client;
    }
    async send(req) {
        const response = await this.client.post(routes_1.ROUTES.notifications.send, req);
        return response.data;
    }
}
class Recipients {
    constructor(client) {
        this.client = client;
        this.notifications = new RecipientsNotifications(client);
        this.preferences = new RecipientsPreferences(client);
    }
    async create(req) {
        const response = await this.client.post(routes_1.ROUTES.recipients.create, req);
        return response.data;
    }
    async createBatch(req) {
        const response = await this.client.post(routes_1.ROUTES.recipients.createBatch, req);
        return response.data;
    }
    async get(recipientID) {
        const response = await this.client.get(routes_1.ROUTES.recipients.get(recipientID));
        return response.data;
    }
    async update(recipientID, req) {
        const response = await this.client.patch(routes_1.ROUTES.recipients.update(recipientID), req);
        return response.data;
    }
    async delete(recipientID) {
        await this.client.delete(routes_1.ROUTES.recipients.delete(recipientID));
    }
}
class RecipientsNotifications {
    constructor(client) {
        this.client = client;
    }
    async list(recipientID, req) {
        const response = await this.client.get(routes_1.ROUTES.recipients.notifications.list(recipientID), {
            params: req,
        });
        return response.data;
    }
    async unreadCount(recipientID) {
        const response = await this.client.get(routes_1.ROUTES.recipients.notifications.unreadCount(recipientID));
        return response.data;
    }
    async updateState(recipientID, req) {
        const response = await this.client.patch(routes_1.ROUTES.recipients.notifications.udpateState(recipientID), req);
        return response.data;
    }
    async delete(recipientID, req) {
        const response = await this.client.delete(routes_1.ROUTES.recipients.notifications.delete(recipientID), {
            data: req,
        });
        return response.data;
    }
}
class RecipientsPreferences {
    constructor(client) {
        this.client = client;
    }
    async list(recipientID) {
        const response = await this.client.get(routes_1.ROUTES.recipients.preferences.list(recipientID));
        return response.data;
    }
    async set(recipientID, req) {
        const response = await this.client.patch(routes_1.ROUTES.recipients.preferences.set(recipientID), {
            target: req.target,
            state: req.state,
        });
        return response.data;
    }
    async check(recipientID, req) {
        const response = await this.client.get(routes_1.ROUTES.recipients.preferences.check(recipientID), {
            params: req.target,
        });
        return response.data;
    }
}
//# sourceMappingURL=bodhveda.js.map