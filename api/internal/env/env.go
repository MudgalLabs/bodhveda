package env

import (
	"os"

	"github.com/joho/godotenv"
)

var (
	APIEnv   string
	LogLevel string
	LogFile  string
	DBURL    string
	WebURL   string
	// ENABLE_SIGN_UP       bool
	// ENABLE_SIGN_IN       bool
	ENABLE_GOOGLE_OAUTH  bool
	GOOGLE_REDIRECT_URL  string
	GOOGLE_CLIENT_ID     string
	GOOGLE_CLIENT_SECRET string
	CipherKey            string
	HashKey              string
)

func IsProd() bool {
	return APIEnv == "production"
}

// Init loads the environment variables from the .env file.
// When running in Docker, the environment variables are loaded by Docker from .env at the root.
func Init(path string) {

	// We load the .env file from the parent directory just in case we are running in development mode.
	// This is because the .env file is in the root of the project.
	err := godotenv.Load(path)

	APIEnv = os.Getenv("BODHVEDA_API_ENV")
	LogLevel = os.Getenv("BODHVEDA_API_LOG_LEVEL")
	LogFile = os.Getenv("BODHVEDA_API_LOG_FILE")
	DBURL = os.Getenv("BODHVEDA_DB_URL")
	WebURL = os.Getenv("BODHVEDA_WEB_URL")
	// ENABLE_SIGN_UP = os.Getenv("BODHVEDA_ENABLE_SIGN_UP") == "true"
	// ENABLE_SIGN_IN = os.Getenv("BODHVEDA_ENABLE_SIGN_IN") == "true"
	ENABLE_GOOGLE_OAUTH = os.Getenv("BODHVEDA_ENABLE_GOOGLE_OAUTH") == "true"
	GOOGLE_REDIRECT_URL = os.Getenv("BODHVEDA_GOOGLE_REDIRECT_URL")
	GOOGLE_CLIENT_ID = os.Getenv("BODHVEDA_GOOGLE_CLIENT_ID")
	GOOGLE_CLIENT_SECRET = os.Getenv("BODHVEDA_GOOGLE_CLIENT_SECRET")
	CipherKey = os.Getenv("BODHVEDA_API_CIPHER_KEY")
	HashKey = os.Getenv("BODHVEDA_API_HASH_KEY")

	// TODO: We should validate the environment variables here to ensure they are set correctly.

	// If APP_ENV is not set, we either missed it in the .env file or the .env file was not loaded
	// by Docker or by the application above. That's why we check if APP_ENV is empty, then
	// if we got error loading the .env file, if both are true, we panic with an error message.
	if APIEnv == "" {
		if err != nil {
			panic("Error loading .env file: " + err.Error())
		}
	}
}
