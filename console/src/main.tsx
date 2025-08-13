import { StrictMode } from "react";
import ReactDOM from "react-dom/client";
import { ToastProvider } from "netra";

import { App } from "@/App";

import "netra/styles.css";

const rootElement = document.getElementById("root")!;
if (!rootElement.innerHTML) {
    const root = ReactDOM.createRoot(rootElement);
    root.render(
        <StrictMode>
            <ToastProvider />
            <App />
        </StrictMode>
    );
}
