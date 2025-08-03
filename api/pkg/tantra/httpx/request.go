package httpx

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi"
)

func ParamInt(r *http.Request, key string) (int, error) {
	return strconv.Atoi(chi.URLParam(r, key))
}
