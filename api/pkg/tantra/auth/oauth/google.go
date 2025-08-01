package oauth

import (
	"encoding/json"
	"io"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

var GoogleConfig *oauth2.Config

func InitGoogle(clientID, clientSecret, redirectURL string) {
	// TOOD: Move this check to `env.go`
	if clientID == "" || clientSecret == "" {
		panic("GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET is not set in environment variables")
	}

	// TOOD: Move this check to `env.go`
	if redirectURL == "" {
		panic("GOOGLE_REDIRECT_URL is not set in environment variables")
	}

	GoogleConfig = &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes:       []string{"email", "profile"},
		Endpoint:     google.Endpoint,
	}
}

type GoogleUserInfo struct {
	Email         string `json:"email"`
	Name          string `json:"name"`
	AvatarURL     string `json:"picture"`
	VerifiedEmail bool   `json:"verified_email"`
}

func ParseGoogleUserInfo(body io.ReadCloser) (*GoogleUserInfo, error) {
	var userInfo GoogleUserInfo
	if err := json.NewDecoder(body).Decode(&userInfo); err != nil {
		return nil, err
	}

	return &userInfo, nil
}
