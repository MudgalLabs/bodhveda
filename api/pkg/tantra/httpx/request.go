package httpx

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi"
	"github.com/gorilla/schema"
)

var decoder = schema.NewDecoder()

func init() {
	decoder.IgnoreUnknownKeys(true)
	decoder.SetAliasTag("schema")
}

// DecodeQuery decodes query params into the given struct pointer.
func DecodeQuery(r *http.Request, dst any) error {
	return decoder.Decode(dst, r.URL.Query())
}

// QueryStr returns the raw string value of a query parameter.
// If not present, returns the empty string.
func QueryStr(r *http.Request, key string) string {
	return r.URL.Query().Get(key)
}

// QueryInt parses the query parameter into an int.
// Returns an error if parsing fails.
func QueryInt(r *http.Request, key string) (int, error) {
	return strconv.Atoi(r.URL.Query().Get(key))
}

// QueryBool parses the query parameter into a bool.
// Returns an error if parsing fails.
func QueryBool(r *http.Request, key string) (bool, error) {
	return strconv.ParseBool(r.URL.Query().Get(key))
}

// ParamStr returns the string value of a URL path parameter.
func ParamStr(r *http.Request, key string) string {
	return chi.URLParam(r, key)
}

// ParamInt parses the URL path parameter into an int.
// Returns an error if parsing fails.
func ParamInt(r *http.Request, key string) (int, error) {
	return strconv.Atoi(chi.URLParam(r, key))
}
