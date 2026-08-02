/**
 * What "mandatory" means, written once and shown wherever the control is.
 *
 * It lives in a tooltip rather than inline body copy because it explains a
 * consequence rather than labelling a field, and because the create and edit
 * modals must not drift apart on it — a recipient who believes they opted out
 * of something they cannot opt out of is the exact failure this setting causes
 * when it is misunderstood.
 */
export function MandatoryHelp() {
    return (
        <div className="space-y-2">
            <p>
                Recipients can't switch a mandatory preference off. Use it for
                password resets, security alerts and other messages that have to
                arrive.
            </p>
            <p>
                The default still applies, so setting it to Disabled stops the
                notification for everyone. Mandatory removes the recipient's
                choice, not yours.
            </p>
        </div>
    );
}
