import { Bodhveda } from "bodhveda";
type BodhvedaProviderProps = {
    bodhveda: Bodhveda;
    recipientID: string;
    children: React.ReactNode;
} | {
    apiKey: string;
    recipientID: string;
    options?: {
        apiURL?: string;
    };
    children: React.ReactNode;
};
export declare function BodhvedaProvider(props: BodhvedaProviderProps): import("react/jsx-runtime").JSX.Element;
export {};
//# sourceMappingURL=provider.d.ts.map