import { jsx as _jsx } from "react/jsx-runtime";
import { useMemo } from "react";
import { Bodhveda } from "bodhveda";
import { BodhvedaContext } from "./context";
export function BodhvedaProvider(props) {
    const hasClient = "bodhveda" in props;
    const hasApiKey = "apiKey" in props;
    const hasOptions = "options" in props;
    if (hasClient && (hasApiKey || hasOptions)) {
        throw new Error("BodhvedaProvider: Provide either `bodhveda` OR `apiKey`/`options`, not both.");
    }
    if (!hasClient && !hasApiKey) {
        throw new Error("BodhvedaProvider: You must provide either a `bodhveda` client OR `apiKey`.");
    }
    const bodhveda = useMemo(() => {
        var _a;
        if ("bodhveda" in props) {
            return props.bodhveda;
        }
        return new Bodhveda(props.apiKey, {
            apiURL: (_a = props.options) === null || _a === void 0 ? void 0 : _a.apiURL,
        });
    }, [
        // Only depend on relevant props based on which variant is active
        "bodhveda" in props ? props.bodhveda : props.apiKey,
        "bodhveda" in props ? undefined : props.options,
    ]);
    return (_jsx(BodhvedaContext.Provider, { value: { bodhveda, recipientID: props.recipientID }, children: props.children }));
}
//# sourceMappingURL=provider.js.map