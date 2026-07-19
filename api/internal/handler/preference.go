package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/mudgallabs/bodhveda/internal/middleware"
	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/service"
	"github.com/mudgallabs/tantra/httpx"
	"github.com/mudgallabs/tantra/jsonx"
)

type CheckRecipientTargetQuery struct {
	Channel string `schema:"channel"`
	Topic   string `schema:"topic"`
	Event   string `schema:"event"`
}

func CreateProjectPreference(s *service.PreferenceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		projectID, err := httpx.ParamInt(r, "project_id")
		if err != nil {
			httpx.BadRequestResponse(w, r, errors.New("Invalid project ID"))
			return
		}

		var payload dto.CreateProjectPreferencePayload
		if err := jsonx.DecodeJSONRequest(&payload, r); err != nil {
			httpx.MalformedJSONResponse(w, r, err)
			return
		}

		payload.ProjectID = projectID

		result, errKind, err := s.CreateProjectPreference(ctx, payload)
		if err != nil {
			httpx.ServiceErrResponse(w, r, errKind, err)
			return
		}

		httpx.SuccessResponse(w, r, http.StatusCreated, "Project preference created", result)
	}
}

// UpdateProjectPreference is the console's catalog edit (project_id in path).
// Only the mutable fields change — name, description and the project-level default — since
// the natural key (channel, topic, event, medium) is immutable. It reuses the
// same service method as the Developer API's UpdateProjectPreferenceAPI.
func UpdateProjectPreference(s *service.PreferenceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		projectID, err := httpx.ParamInt(r, "project_id")
		if err != nil {
			httpx.BadRequestResponse(w, r, errors.New("Invalid project ID"))
			return
		}

		preferenceID, err := httpx.ParamInt(r, "preference_id")
		if err != nil {
			httpx.BadRequestResponse(w, r, errors.New("Invalid preference ID"))
			return
		}

		var payload dto.UpdateProjectPreferencePayload
		if err := jsonx.DecodeJSONRequest(&payload, r); err != nil {
			httpx.MalformedJSONResponse(w, r, err)
			return
		}

		result, errKind, err := s.UpdateProjectPreference(ctx, projectID, preferenceID, payload)
		if err != nil {
			httpx.ServiceErrResponse(w, r, errKind, err)
			return
		}

		httpx.SuccessResponse(w, r, http.StatusOK, "Project preference updated", result)
	}
}

// The handlers below (…API suffix) are the Developer API's project-preference
// (catalog) CRUD. They are project-scoped by the API key — there is no
// project_id in the path — mirroring the rest of the Developer API. The console
// keeps its own project_id-in-path handlers (CreateProjectPreference,
// ListPreferences, DeletePreference) unchanged.

func ListProjectPreferencesAPI(s *service.PreferenceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		apiKey := middleware.GetAPIKeyFromContext(ctx)

		result, errKind, err := s.ListProjectPreferencesForAPI(ctx, apiKey.ProjectID)
		if err != nil {
			httpx.ServiceErrResponse(w, r, errKind, err)
			return
		}

		httpx.SuccessResponse(w, r, http.StatusOK, "", result)
	}
}

func CreateProjectPreferenceAPI(s *service.PreferenceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		apiKey := middleware.GetAPIKeyFromContext(ctx)

		var payload dto.CreateProjectPreferencePayload
		if err := jsonx.DecodeJSONRequest(&payload, r); err != nil {
			httpx.MalformedJSONResponse(w, r, err)
			return
		}

		payload.ProjectID = apiKey.ProjectID

		result, errKind, err := s.CreateProjectPreference(ctx, payload)
		if err != nil {
			httpx.ServiceErrResponse(w, r, errKind, err)
			return
		}

		httpx.SuccessResponse(w, r, http.StatusCreated, "Project preference created", result)
	}
}

// UpsertProjectPreferencesAPI is the declarative bulk catalog setup: the body is
// an ARRAY of project preferences, merged by natural key. `?prune=true` also
// removes catalog rows absent from the array (default: merge, leaving them).
func UpsertProjectPreferencesAPI(s *service.PreferenceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		apiKey := middleware.GetAPIKeyFromContext(ctx)

		var items []dto.CreateProjectPreferencePayload
		if err := jsonx.DecodeJSONRequest(&items, r); err != nil {
			httpx.MalformedJSONResponse(w, r, err)
			return
		}

		// prune is optional; missing means merge (false). Only a malformed value
		// is an error — QueryBool would reject an absent param.
		prune := false
		if raw := httpx.QueryStr(r, "prune"); raw != "" {
			v, err := strconv.ParseBool(raw)
			if err != nil {
				httpx.BadRequestResponse(w, r, errors.New("Invalid 'prune' query param; expected true or false"))
				return
			}
			prune = v
		}

		result, errKind, err := s.UpsertProjectPreferences(ctx, apiKey.ProjectID, items, prune)
		if err != nil {
			httpx.ServiceErrResponse(w, r, errKind, err)
			return
		}

		httpx.SuccessResponse(w, r, http.StatusOK, "Project preferences upserted", result)
	}
}

func GetProjectPreferenceAPI(s *service.PreferenceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		apiKey := middleware.GetAPIKeyFromContext(ctx)

		preferenceID, err := httpx.ParamInt(r, "preference_id")
		if err != nil {
			httpx.BadRequestResponse(w, r, errors.New("Invalid preference ID"))
			return
		}

		result, errKind, err := s.GetProjectPreference(ctx, apiKey.ProjectID, preferenceID)
		if err != nil {
			httpx.ServiceErrResponse(w, r, errKind, err)
			return
		}

		httpx.SuccessResponse(w, r, http.StatusOK, "", result)
	}
}

func UpdateProjectPreferenceAPI(s *service.PreferenceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		apiKey := middleware.GetAPIKeyFromContext(ctx)

		preferenceID, err := httpx.ParamInt(r, "preference_id")
		if err != nil {
			httpx.BadRequestResponse(w, r, errors.New("Invalid preference ID"))
			return
		}

		var payload dto.UpdateProjectPreferencePayload
		if err := jsonx.DecodeJSONRequest(&payload, r); err != nil {
			httpx.MalformedJSONResponse(w, r, err)
			return
		}

		result, errKind, err := s.UpdateProjectPreference(ctx, apiKey.ProjectID, preferenceID, payload)
		if err != nil {
			httpx.ServiceErrResponse(w, r, errKind, err)
			return
		}

		httpx.SuccessResponse(w, r, http.StatusOK, "Project preference updated", result)
	}
}

func DeleteProjectPreferenceAPI(s *service.PreferenceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		apiKey := middleware.GetAPIKeyFromContext(ctx)

		preferenceID, err := httpx.ParamInt(r, "preference_id")
		if err != nil {
			httpx.BadRequestResponse(w, r, errors.New("Invalid preference ID"))
			return
		}

		errKind, err := s.DeleteProjectPreference(ctx, apiKey.ProjectID, preferenceID)
		if err != nil {
			httpx.ServiceErrResponse(w, r, errKind, err)
			return
		}

		httpx.SuccessResponse(w, r, http.StatusOK, "", nil)
	}
}

func ListPreferences(s *service.PreferenceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		projectID, err := httpx.ParamInt(r, "project_id")
		if err != nil {
			httpx.BadRequestResponse(w, r, errors.New("Invalid project ID"))
			return
		}

		kind := httpx.QueryStr(r, "kind")

		if kind == "" {
			// Default to project if not specified
			kind = "project"
		}

		if kind != "project" && kind != "recipient" {
			httpx.BadRequestResponse(w, r, errors.New("Invalid preference kind"))
			return
		}

		switch kind {
		case "project":
			result, errKind, err := s.ListProjectPreferences(ctx, projectID)
			if err != nil {
				httpx.ServiceErrResponse(w, r, errKind, err)
				return
			}

			httpx.SuccessResponse(w, r, http.StatusOK, "", result)
			return
		case "recipient":
			result, errKind, err := s.ListRecipientPreferences(ctx, projectID)
			if err != nil {
				httpx.ServiceErrResponse(w, r, errKind, err)
				return
			}

			httpx.SuccessResponse(w, r, http.StatusOK, "", result)
			return
		}

		httpx.BadRequestResponse(w, r, errors.New("Invalid preference kind"))
	}
}

func UpsertRecipientPreferences(s *service.PreferenceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		projectID, err := httpx.ParamInt(r, "project_id")
		if err != nil {
			httpx.BadRequestResponse(w, r, errors.New("Invalid project ID"))
			return
		}

		recipientExtID := strings.ToLower(httpx.ParamStr(r, "recipient_external_id"))

		var payload dto.UpsertRecipientPreferencePayload
		if err := jsonx.DecodeJSONRequest(&payload, r); err != nil {
			httpx.MalformedJSONResponse(w, r, err)
			return
		}

		payload.ProjectID = projectID
		payload.RecipientExtID = recipientExtID

		result, errKind, err := s.UpsertRecipientPreference(ctx, payload)
		if err != nil {
			httpx.ServiceErrResponse(w, r, errKind, err)
			return
		}

		httpx.SuccessResponse(w, r, http.StatusCreated, "Recipient preference updated", result)
	}
}

func GetRecipientProjectPreferences(s *service.PreferenceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		apiKey := middleware.GetAPIKeyFromContext(ctx)

		recipientExtID := strings.ToLower(httpx.ParamStr(r, "recipient_external_id"))
		if recipientExtID == "" {
			httpx.BadRequestResponse(w, r, errors.New("recipient_id required"))
			return
		}

		result, errKind, err := s.GetRecipientProjectPreferences(ctx, apiKey.ProjectID, recipientExtID)
		if err != nil {
			httpx.ServiceErrResponse(w, r, errKind, err)
			return
		}

		httpx.SuccessResponse(w, r, http.StatusOK, "", result)
	}
}

// GetRecipientPreferencesConsole is the console's per-recipient preference read:
// every (target, Active medium) resolved by the SAME cascade the send path uses.
//
// It shares that resolver with the Developer API's read
// (GetRecipientProjectPreferences) — the two agree on every value, and a test
// pins both against the send path's gating. They stay separate service methods
// only because this one additionally exposes `source` (the cascade rung) and
// 404s for an unknown recipient, neither of which belongs on the public surface:
// `source` would freeze the resolver's vocabulary into a permanent contract, and
// the Dev API's routes auto-create the recipient so a 404 is unreachable there.
// See the Phase 9.3.1 deviations in agent-docs/overview.md.
func GetRecipientPreferencesConsole(s *service.PreferenceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		projectID, err := httpx.ParamInt(r, "project_id")
		if err != nil {
			httpx.BadRequestResponse(w, r, errors.New("Invalid project ID"))
			return
		}

		recipientExtID := strings.ToLower(httpx.ParamStr(r, "recipient_external_id"))
		if recipientExtID == "" {
			httpx.BadRequestResponse(w, r, errors.New("recipient_external_id required"))
			return
		}

		result, errKind, err := s.ResolveRecipientPreferences(ctx, projectID, recipientExtID)
		if err != nil {
			httpx.ServiceErrResponse(w, r, errKind, err)
			return
		}

		httpx.SuccessResponse(w, r, http.StatusOK, "", result)
	}
}

func UpdateRecipientPreferenceForTarget(s *service.PreferenceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		apiKey := middleware.GetAPIKeyFromContext(ctx)
		recipientExtID := strings.ToLower(httpx.ParamStr(r, "recipient_external_id"))
		if recipientExtID == "" {
			httpx.BadRequestResponse(w, r, errors.New("recipient_id required"))
			return
		}

		var req dto.PatchRecipientPreferenceTargetPayload
		if err := jsonx.DecodeJSONRequest(&req, r); err != nil {
			httpx.MalformedJSONResponse(w, r, err)
			return
		}

		result, errKind, err := s.UpdateRecipientPreferenceTarget(ctx, apiKey.ProjectID, recipientExtID, req)
		if err != nil {
			httpx.ServiceErrResponse(w, r, errKind, err)
			return
		}

		httpx.SuccessResponse(w, r, http.StatusOK, "", result)
	}
}

func CheckRecipientPreferenceForTarget(s *service.PreferenceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		apiKey := middleware.GetAPIKeyFromContext(ctx)
		recipientExtID := strings.ToLower(httpx.ParamStr(r, "recipient_external_id"))
		if recipientExtID == "" {
			httpx.BadRequestResponse(w, r, errors.New("recipient_id required"))
			return
		}

		var payload dto.CheckRecipientTargetPayload
		if err := httpx.DecodeQuery(r, &payload); err != nil {
			httpx.BadRequestResponse(w, r, err)
			return
		}

		result, errKind, err := s.CheckRecipientTargetSubscription(ctx, apiKey.ProjectID, recipientExtID, payload)
		if err != nil {
			httpx.ServiceErrResponse(w, r, errKind, err)
			return
		}

		httpx.SuccessResponse(w, r, http.StatusOK, "", result)
	}
}

func DeletePreference(s *service.PreferenceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		projectID, err := httpx.ParamInt(r, "project_id")
		if err != nil {
			httpx.BadRequestResponse(w, r, errors.New("Invalid project ID"))
			return
		}

		preferenceID, err := httpx.ParamInt(r, "preference_id")
		if err != nil {
			httpx.BadRequestResponse(w, r, errors.New("Invalid preference ID"))
			return
		}

		payload := &dto.DeletePreferencePayload{
			ProjectID:    projectID,
			PreferenceID: preferenceID,
		}

		errKind, err := s.Delete(ctx, payload)
		if err != nil {
			httpx.ServiceErrResponse(w, r, errKind, err)
			return
		}

		httpx.SuccessResponse(w, r, http.StatusOK, "", nil)
	}
}
