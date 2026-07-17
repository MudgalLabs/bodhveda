import { createContext } from "react";
import { Bodhveda } from "@bodhveda/js";

interface BodhvedaContextType {
    bodhveda: Bodhveda;
    recipientID: string;
}

export const BodhvedaContext = createContext<BodhvedaContextType | null>(null);
