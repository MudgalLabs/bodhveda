import { useCallback, useState } from "react";

export function usePortalRef() {
    const [portalEl, setPortalEl] = useState<HTMLElement | null>(null);

    const handleRef = useCallback((node: HTMLElement | null) => {
        if (node) setPortalEl(node);
    }, []);

    return { portalEl, handleRef };
}
