import { createFileRoute } from "@tanstack/react-router";

import { APIKeyList } from "@/features/api_key/list/api_key_list";

export const Route = createFileRoute("/projects/$id/api-keys")({
    component: APIKeyList,
});
