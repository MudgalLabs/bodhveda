import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import { tanstackRouter } from "@tanstack/router-plugin/vite";
import tailwindcss from "@tailwindcss/vite";

export default ({ mode }: { mode: string }) => {
    return defineConfig({
        plugins: [
            tanstackRouter({
                target: "react",
                autoCodeSplitting: true,
            }),
            react(),
            tailwindcss(),
        ],
        server: {
            host: "localhost",
            port: 6970,
        },
        resolve: {
            alias: {
                "@": "/src",
            },
        },
        envPrefix: "BODHVEDA_",
        envDir: mode === "development" ? "../" : undefined,
    });
};
