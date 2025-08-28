import { AnyUseMutationOptions, AnyUseQueryOptions, UseQueryResult } from "@tanstack/react-query";
import { ListNotificationsResponse, ListPreferencesResponse, UpdateNotificationsStateRequest, UpdateNotificationsStateResponse, SetPreferenceRequest, SetPreferenceResponse, DeleteNotificationsResponse, DeleteNotificationsRequest, Target, CheckPreferenceResponse } from "bodhveda";
type QueryOptions = Omit<AnyUseQueryOptions, "queryKey">;
type MutationOptions = AnyUseMutationOptions;
export declare function useBodhveda(): import("bodhveda").Bodhveda;
export declare function useRecipientID(): string;
export declare function useNotifications(options?: QueryOptions): UseQueryResult<ListNotificationsResponse>;
export declare function useNotificationsUnreadCount(options?: QueryOptions): UseQueryResult<{
    unread_count: number;
}>;
export declare function useUpdateNotificationsState(options?: MutationOptions): import("@tanstack/react-query").UseMutationResult<UpdateNotificationsStateResponse, unknown, UpdateNotificationsStateRequest, unknown>;
export declare function useDeleteNotifications(options?: MutationOptions): import("@tanstack/react-query").UseMutationResult<DeleteNotificationsResponse, unknown, DeleteNotificationsRequest, unknown>;
export declare function usePreferences(options?: QueryOptions): UseQueryResult<ListPreferencesResponse>;
export declare function useUpdatePreference(options?: MutationOptions): import("@tanstack/react-query").UseMutationResult<SetPreferenceResponse, unknown, SetPreferenceRequest, unknown>;
export declare function useCheckPreference(target: Target, options?: QueryOptions): UseQueryResult<CheckPreferenceResponse>;
export {};
//# sourceMappingURL=hooks.d.ts.map