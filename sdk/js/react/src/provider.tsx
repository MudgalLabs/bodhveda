import { useMemo } from "react";
import { Bodhveda } from "bodhveda";

import { BodhvedaContext } from "./context";

type BodhvedaProviderProps =
    | {
          bodhveda: Bodhveda;
          recipientID: string;
          children: React.ReactNode;
      }
    | {
          apiKey: string;
          recipientID: string;
          options?: {
              apiURL?: string;
          };
          children: React.ReactNode;
      };

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
