import { createFileRoute } from "@tanstack/react-router";
import { ProjectPreferenceList } from "@/features/preference/list/preference_list";

export const Route = createFileRoute("/projects/$id/preferences")({
    component: ProjectPreferenceList,
});
