import { FC } from "react";
import { IconArrowUpRight } from "netra";

import { cn } from "@/lib/utils";

interface BuilderCardProps {
    className?: string;
    /** Builder name shown in bold. */
    name?: string;
    /** One-line tagline shown under the name. */
    tagline?: string;
    /** Where the card links to. */
    href?: string;
    /** Avatar photo URL. */
    avatarSrc?: string;
}

// BuilderCard is the brand credit shown on the sign-in screen (and anywhere a
// "built by" credit fits). It points to ceoshikhar.com, the home of all the
// products. Defaults are baked in but every field is overridable so the same
// card can be reused across products.
export const BuilderCard: FC<BuilderCardProps> = ({
    className,
    name = "ceoshikhar.com",
    tagline = "I build things. Sometimes they're good.",
    href = "https://ceoshikhar.com",
    avatarSrc = "https://ceoshikhar.com/images/me.png",
}) => {
    return (
        <a
            href={href}
            target="_blank"
            rel="noopener noreferrer"
            aria-label={`Built by ${name} — ${tagline}`}
            className={cn(
                "link-unstyled group border-border-subtle hover:border-link relative flex items-center gap-x-3 rounded-md border p-3 transition-all duration-200 ease-out hover:-translate-y-1",
                className
            )}
        >
            <img
                src={avatarSrc}
                alt={name}
                width={44}
                height={44}
                className="border-border-subtle size-11 shrink-0 rounded-full border object-cover"
            />

            <div className="flex min-w-0 flex-col pr-5 text-left">
                <span className="text-text-muted text-xs leading-tight">
                    Built by one person
                </span>
                <span className="text-link text-sm leading-snug font-semibold">
                    {name}
                </span>
                <span className="text-text-muted truncate text-xs leading-tight">
                    {tagline}
                </span>
            </div>

            <IconArrowUpRight
                size={18}
                className="text-text-muted group-hover:text-foreground absolute top-3 right-3 transition-all duration-200 ease-out group-hover:-translate-y-1 group-hover:translate-x-1"
            />
        </a>
    );
};
