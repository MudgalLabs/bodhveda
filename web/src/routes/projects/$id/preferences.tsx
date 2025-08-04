import { createFileRoute } from "@tanstack/react-router";
import { ProjectPreferenceList } from "@/features/project_preference/list/project_preference_list";

export const Route = createFileRoute("/projects/$id/preferences")({
    component: ProjectPreferenceList,
});
