import { useMemo } from "react";
import { Bodhveda } from "bodhveda";

import { BodhvedaContext } from "./context";

/**
 * Props for {@link BodhvedaProvider}.
 *
 * You must provide `recipientID` along with either a `bodhveda` client instance, or an `apiKey` (with optional `options`).
 *
 * @example
 * ```tsx
 * <BodhvedaProvider bodhveda={bodhvedaClient} recipientID="user-123">
 *   <App />
 * </BodhvedaProvider>
 * ```
 *
 * @example
 * ```tsx
 * <BodhvedaProvider apiKey="your-api-key" recipientID="user-123">
 *   <App />
 * </BodhvedaProvider>
 * ```
 */
type BodhvedaProviderProps =
    | {
          /**
           * An existing Bodhveda client instance.
           */
          bodhveda: Bodhveda;
          /**
           * The recipient's ID.
           */
          recipientID: string;
          /**
           * React children.
           */
          children: React.ReactNode;
      }
    | {
          /**
           * API key for Bodhveda.
           */
          apiKey: string;
          /**
           * The recipient's ID.
           */
          recipientID: string;
          /**
           * Optional configuration.
           */
          options?: {
              /**
               * Override the API URL.
               */
              apiURL?: string;
          };
          /**
           * React children.
           */
          children: React.ReactNode;
      };

/**
 * Use this at the root of your React app to be able to use the hooks provided.
 *
 * @param props - {@link BodhvedaProviderProps}
 * @returns React provider component.
 */
export function BodhvedaProvider(props: BodhvedaProviderProps) {
    const hasClient = "bodhveda" in props;
    const hasApiKey = "apiKey" in props;
    const hasOptions = "options" in props;

    if (hasClient && (hasApiKey || hasOptions)) {
        throw new Error(
            "BodhvedaProvider: Provide either `bodhveda` OR `apiKey`/`options`, not both."
        );
    }

    if (!hasClient && !hasApiKey) {
        throw new Error(
            "BodhvedaProvider: You must provide either a `bodhveda` client OR `apiKey`."
        );
    }

    const bodhveda = useMemo(() => {
        if ("bodhveda" in props) {
            return props.bodhveda;
        }

        return new Bodhveda(props.apiKey, {
            apiURL: props.options?.apiURL,
        });
    }, [
        // Only depend on relevant props based on which variant is active
        "bodhveda" in props ? props.bodhveda : props.apiKey,
        "bodhveda" in props ? undefined : props.options,
    ]);

    return (
        <BodhvedaContext.Provider
            value={{ bodhveda, recipientID: props.recipientID }}
        >
            {props.children}
        </BodhvedaContext.Provider>
    );
}
