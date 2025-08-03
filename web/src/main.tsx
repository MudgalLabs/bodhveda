import { StrictMode } from "react";
import ReactDOM from "react-dom/client";

import { App } from "@/App";

import "netra/styles.css";
import { ToastProvider } from "netra";

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
