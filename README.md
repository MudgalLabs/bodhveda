<p align="center">
  <img src="./.github/screenshots/banner.png" alt="Bodhveda banner" />
</p>

# Bodhveda

[Bodhveda](https://bodhveda.com) is an open-source notification platform for your app – like email, but for in-app experiences.

Built for indie hackers, startups, and modern SaaS, Bodhveda helps you:

-   📬 Send direct or broadcast notifications
-   📦 Fetch and manage in-app notifications
-   🔒 Stay in control with self-hosted or fully managed deployment

No need to pollute your core database. No need to build notification infrastructure from scratch.

**You send. We deliver.**

## 🚀 Quick Features

-   📦 SDKs for Node.js and Go
-   🧠 Lazy materialization - broadcasts turn into notifications only on fetch which reduces your notification per month usage.
-   📊 Basic analytics and usage tracking (coming soon)
-   🧱 Fully open-source and self-hostable via Docker

## 🧪 SDK Quick Start

### Node.js

```ts
import { Bodhveda } from "bodhveda";

const bodhveda = new Bodhveda("YOUR_API_KEY");

const notification = await bodhveda.notifications.send("recipient_123", {
    title: "Welcome!",
    type: "info",
});

const notifications = await bodhveda.notifications.fetch("recipient_123");
```

### Go

```go
import "github.com/mudgallabs/bodhveda"

bodhveda := bodhveda.New("YOUR_API_KEY")

notification, err := bodhveda.Notifications.Send("recipient_123", map[string]interface{}{
    "title": "Welcome!",
    "type":  "info",
})

notifications, err := bodhveda.Notifications.Fetch("recipient_123")
```

---

## 📚 Documentation

-   REST API → [`docs/REST_API.md`](docs/REST_API.md)
-   Node SDK → [`docs/NODE_SDK.md`](docs/NODE_SDK.md)
-   Go SDK → [`docs/GO_SDK.md`](docs/GO_SDK.md)

> Full developer docs will be live at [bodhveda.com/docs](https://bodhveda.com/docs) soon.

---

## 🧱 Project Structure

```
.
├── api/           → Go backend (core logic + API)
├── web/           → React web dashboard (coming soon)
├── docs/          → SDK + API documentation
├── migrations/    → PostgreSQL schema (Goose)
```

---

## 🐳 Self-Host (Local Dev)

Spin up the dev environment:

```bash
git clone https://github.com/mudgallabs/bodhveda
cd bodhveda
cp .env.example .env
make dev
```

Runs:

-   Go API on `http://localhost:1337`
-   Postgres via Docker Compose

## 🚣️ Roadmap

-   [x] Send direct and broadcast notifications
-   [x] Fetch and manage notifications
-   [x] SDKs (Node, Go)
-   [ ] Dashboard (projects, API keys, usage, logs, analytics)

## 📜 License

[AGPL v3](LICENSE) because notifications should be free to own, run, and customize.

<p align="center">
  Built with ❤️ by <a href="https://mudgallabs.com" target="_blank">Mudgal Labs</a>
</p>
