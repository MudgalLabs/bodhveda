package httpx

import (
	"net/http"

	"github.com/mudgallabs/tantra/apires"
	"github.com/mudgallabs/tantra/jsonx"
	"github.com/mudgallabs/tantra/logger"
	"github.com/mudgallabs/tantra/service"
)

func SuccessResponse(w http.ResponseWriter, r *http.Request, statusCode int, message string, data any) {
	l := logger.FromCtx(r.Context())
	// Not logging data to avoid unnecessary exposure and log size increase.
	l.Debugw("success response", "message", message)
	jsonx.WriteJSONResponse(w, statusCode, apires.Success(statusCode, message, data))
}

func InternalServerErrorResponse(w http.ResponseWriter, r *http.Request, err error) {
	l := logger.FromCtx(r.Context())
	l.Errorw("internal error response", "error", err.Error())
	jsonx.WriteJSONResponse(w, http.StatusInternalServerError, apires.InternalError(err))
}

func BadRequestResponse(w http.ResponseWriter, r *http.Request, err error) {
	l := logger.FromCtx(r.Context())
	l.Debugw("bad request response", "error", err)
	jsonx.WriteJSONResponse(w, http.StatusBadRequest, apires.Error(http.StatusBadRequest, err.Error(), nil))
}

func MalformedJSONResponse(w http.ResponseWriter, r *http.Request, err error) {
	l := logger.FromCtx(r.Context())
	l.Debugw("malformed json response", "error", err.Error())
	jsonx.WriteJSONResponse(w, http.StatusBadRequest, apires.MalformedJSONError(err))
}

func InvalidInputResponse(w http.ResponseWriter, r *http.Request, errs service.InputValidationErrors) {
	l := logger.FromCtx(r.Context())
	l.Debugw("invalid input response", "error", errs)
	jsonx.WriteJSONResponse(w, http.StatusBadRequest, apires.InvalidInputError(errs))
}

func ConflictResponse(w http.ResponseWriter, r *http.Request, err error) {
	l := logger.FromCtx(r.Context())
	l.Debugw("conflict response", "error", err)
	jsonx.WriteJSONResponse(w, http.StatusConflict, apires.Error(http.StatusConflict, err.Error(), nil))
}

func NotFoundResponse(w http.ResponseWriter, r *http.Request, err error) {
	l := logger.FromCtx(r.Context())
	l.Debugw("not found response", "error", err)
	jsonx.WriteJSONResponse(w, http.StatusNotFound, apires.Error(http.StatusNotFound, err.Error(), nil))
}

func UnauthorizedErrorResponse(w http.ResponseWriter, r *http.Request, message string, err error) {
	l := logger.FromCtx(r.Context())
	l.Warnw("error response", "message", message, "error", err)

	msg := "unauthorized"
	if message != "" {
		msg = message
	}

	jsonx.WriteJSONResponse(w, http.StatusUnauthorized, apires.Error(http.StatusUnauthorized, msg, nil))
}

// ServiceErrReponse is a helper function that translates Service.ErrKind and error to an HTTP response.
func ServiceErrResponse(w http.ResponseWriter, r *http.Request, errKind service.Error, err error) {
	l := logger.FromCtx(r.Context())

	// Just a safety net.
	if err == nil || errKind == service.ErrNone {
		l.DPanicw("errKind and/or err is not present", "error", err, "errKind", errKind)
		return
	}

	switch {
	case errKind == service.ErrBadRequest:
		BadRequestResponse(w, r, err)
		return

	case errKind == service.ErrUnauthorized:
		UnauthorizedErrorResponse(w, r, err.Error(), err)
		return

	case errKind == service.ErrConflict:
		ConflictResponse(w, r, err)
		return

	case errKind == service.ErrInvalidInput:
		inputValidationErrors, ok := err.(service.InputValidationErrors)

		if !ok {
			inputValidationErrors = service.NewInputValidationErrorsWithError(apires.NewApiError("Something went wrong", err.Error(), "", nil))
		}

		InvalidInputResponse(w, r, inputValidationErrors)
		return

	case errKind == service.ErrNotFound:
		NotFoundResponse(w, r, err)
		return

	case errKind == service.ErrInternalServerError:
		InternalServerErrorResponse(w, r, err)
		return

	default:
		l.DPanicw("reached an unreachable switch-case", "error", err, "errKind", errKind)
	}
}
