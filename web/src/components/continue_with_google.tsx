import { FC } from "react";

import { Button, ButtonProps } from "@/components/button";
import { Google } from "@/components/google";
import { cn, isProd } from "@/lib/utils";

export const ContinueWithGoogle: FC<ButtonProps> = (props) => {
    const { className, ...rest } = props;

    let googleOAuthURL = import.meta.env.BODHVEA_GOOGLE_OAUTH_URL;

    if (!googleOAuthURL) {
        if (isProd()) {
            throw new Error("Google OAuth URL is missing");
        } else {
            googleOAuthURL =
                "http://localhost:1337/v1/platform/auth/oauth/google";
        }
    }

    return (
        <Button
            variant="secondary"
            type="button"
            size="large"
            className={cn("", className)}
            onClick={() => {
                window.location.assign(googleOAuthURL);
            }}
            {...rest}
        >
            <Google />
            Continue with Google
        </Button>
    );
};
