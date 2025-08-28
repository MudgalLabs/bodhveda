import { createContext } from "react";
import { Bodhveda } from "bodhveda";

interface BodhvedaContextType {
    bodhveda: Bodhveda;
    recipientID: string;
}

export const BodhvedaContext = createContext<BodhvedaContextType | null>(null);
