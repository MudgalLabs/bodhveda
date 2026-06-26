import { createFileRoute, redirect } from "@tanstack/react-router";
import { Card, CardContent } from "netra";

import { ContinueWithGoogle } from "@/components/continue_with_google";
import { Branding } from "@/components/branding";
import { BuilderCard } from "@/components/builder_card";

export const Route = createFileRoute("/auth/sign-in")({
    component: Login,
    beforeLoad: ({ context }) => {
        if (context.auth.isAuthenticated) {
            throw redirect({
                to: "/",
            });
        }
    },
});

function Login() {
    return (
        <div className="flex h-dvh w-full flex-col items-center justify-between overflow-auto px-4">
            <div />

            <div className="w-full">
                <Branding
                    className="z-1 flex justify-center"
                    size="default"
                    hideBetaTag
                    hideText
                />

                <div className="h-16" />

                <Card className="mx-auto w-full bg-transparent px-6 py-4 sm:w-fit">
                    <CardContent className="flex flex-col items-center justify-center gap-y-4">
                        <h1 className="heading">Sign in to Bodhveda</h1>

                        <ContinueWithGoogle className="w-full" />
                    </CardContent>
                </Card>

                <div className="h-4" />

                <p className="text-text-muted w-full text-center text-sm text-balance">
                    By continuing, you agree to our{" "}
                    <a
                        href="https://bodhveda.com/terms"
                        target="_blank"
                        rel="noopener noreferrer"
                    >
                        Terms of Service
                    </a>{" "}
                    and{" "}
                    <a
                        href="https://bodhveda.com/privacy"
                        target="_blank"
                        rel="noopener noreferrer"
                    >
                        Privacy Policy
                    </a>
                    .
                </p>
            </div>

            <div className="flex-center py-6 md:py-10">
                <div className="space-y-4 text-center">
                    <BuilderCard className="mx-auto w-full max-w-[420px]" />

                    <p className="text-text-muted text-sm sm:text-base">
                        Give feedback, request a feature, report a bug or{" "}
                        <br className="block sm:hidden" />
                        just say hi on{" "}
                        <a
                            href="mailto:hey@ceoshikhar.com"
                            className="text-sm! font-bold sm:text-base!"
                        >
                            hey@ceoshikhar.com
                        </a>
                    </p>
                </div>
            </div>
        </div>
    );
}
