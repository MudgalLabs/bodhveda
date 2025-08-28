import axios from "axios";
import { ROUTES } from "./routes";
export class Bodhveda {
    constructor(apiKey, options = {}) {
        const { apiURL = "https://api.bodhveda.com" } = options;
        const client = axios.create({
            baseURL: apiURL,
            headers: {
                "Content-Type": "application/json",
                Authorization: `Bearer ${apiKey}`,
            },
        });
        client.interceptors.response.use((response) => response.data, // Unwrap the main data object from the response.
        (error) => {
            var _a, _b;
            if (axios.isAxiosError(error) && ((_a = error.response) === null || _a === void 0 ? void 0 : _a.data)) {
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
class Notifications {
    constructor(client) {
        this.client = client;
    }
    async send(req) {
        const response = await this.client.post(ROUTES.notifications.send, req);
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
        const response = await this.client.post(ROUTES.recipients.create, req);
        return response.data;
    }
    async createBatch(req) {
        const response = await this.client.post(ROUTES.recipients.createBatch, req);
        return response.data;
    }
    async get(recipientID) {
        const response = await this.client.get(ROUTES.recipients.get(recipientID));
        return response.data;
    }
    async update(recipientID, req) {
        const response = await this.client.patch(ROUTES.recipients.update(recipientID), req);
        return response.data;
    }
    async delete(recipientID) {
        await this.client.delete(ROUTES.recipients.delete(recipientID));
    }
}
class RecipientsNotifications {
    constructor(client) {
        this.client = client;
    }
    async list(recipientID, req) {
        const response = await this.client.get(ROUTES.recipients.notifications.list(recipientID), {
            params: req,
        });
        return response.data;
    }
    async unreadCount(recipientID) {
        const response = await this.client.get(ROUTES.recipients.notifications.unreadCount(recipientID));
        return response.data;
    }
    async updateState(recipientID, req) {
        const response = await this.client.patch(ROUTES.recipients.notifications.udpateState(recipientID), req);
        return response.data;
    }
    async delete(recipientID, req) {
        const response = await this.client.delete(ROUTES.recipients.notifications.delete(recipientID), {
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
        const response = await this.client.get(ROUTES.recipients.preferences.list(recipientID));
        return response.data;
    }
    async set(recipientID, req) {
        const response = await this.client.patch(ROUTES.recipients.preferences.set(recipientID), {
            target: req.target,
            state: req.state,
        });
        return response.data;
    }
    async check(recipientID, req) {
        const response = await this.client.get(ROUTES.recipients.preferences.check(recipientID), {
            params: req.target,
        });
        return response.data;
    }
}
//# sourceMappingURL=bodhveda.js.map