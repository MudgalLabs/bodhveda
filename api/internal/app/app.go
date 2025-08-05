package app

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudgallabs/bodhveda/internal/env"
	"github.com/mudgallabs/bodhveda/internal/feature/user_identity"
	"github.com/mudgallabs/bodhveda/internal/feature/user_profile"
	"github.com/mudgallabs/bodhveda/internal/model/repository"
	"github.com/mudgallabs/bodhveda/internal/pg"
	"github.com/mudgallabs/bodhveda/internal/service"
	"github.com/mudgallabs/tantra/auth/oauth"
	"github.com/mudgallabs/tantra/auth/session"
	"github.com/mudgallabs/tantra/dbx"
	"github.com/mudgallabs/tantra/logger"
)

var APP *App

var DB *pgxpool.Pool

type App struct {
	Service    services
	Repository repositories
}

// All the services.
type services struct {
	APIKey            *service.APIKeyService
	Notification      *service.NotificationService
	Project           *service.ProjectService
	Recipient         *service.RecipientService
	UserIdentity      *user_identity.Service
	UserProfile       *user_profile.Service
	ProjectPreference *service.PreferenceService
}

// Access to all repositories for reading.
// Write access only available to services.
type repositories struct {
	APIKey            repository.APIKeyReader
	Notification      repository.NotificationReader
	Project           repository.ProjectReader
	Recipient         repository.RecipientReader
	UserIdentity      user_identity.Reader
	UserProfile       user_profile.Reader
	ProjectPreference repository.PreferenceRepository
}

func Init() {
	env.Init("../.env")

	logger.Init(env.LogLevel, env.LogFile)

	// IDK what this does but it was on the blogpost so I'm using it.
	// I think it has something to do with Go sync for multi threading?
	defer logger.Get().Sync()

	session.Init()

	db, err := dbx.Init(env.DBURL)
	if err != nil {
		panic(err)
	}

	oauth.InitGoogle(env.GOOGLE_CLIENT_ID, env.GOOGLE_CLIENT_SECRET, env.GOOGLE_REDIRECT_URL)

	apikeyRepository := pg.NewAPIKeyRepo(db)
	notificationRepository := pg.NewNotificationRepo(db)
	projectRepository := pg.NewProjectRepo(db)
	recipientRepository := pg.NewRecipientRepo(db)
	userProfileRepository := user_profile.NewRepository(db)
	userIdentityRepository := user_identity.NewRepository(db)
	projectPreferenceRepository := pg.NewPreferenceRepo(db)

	apikeyService := service.NewAPIKeyService(apikeyRepository, projectRepository)
	notificationService := service.NewNotificationService(notificationRepository)
	projectService := service.NewProjectService(projectRepository)
	recipientService := service.NewRecipientService(recipientRepository)
	userIdentityService := user_identity.NewService(userIdentityRepository, userProfileRepository)
	userProfileService := user_profile.NewService(userProfileRepository)
	projectPreferenceService := service.NewProjectPreferenceService(projectPreferenceRepository)

	services := services{
		APIKey:            apikeyService,
		Notification:      notificationService,
		Project:           projectService,
		Recipient:         recipientService,
		UserIdentity:      userIdentityService,
		UserProfile:       userProfileService,
		ProjectPreference: projectPreferenceService,
	}

	repositories := repositories{
		APIKey:            apikeyRepository,
		Notification:      notificationRepository,
		Project:           projectRepository,
		Recipient:         recipientRepository,
		UserIdentity:      userIdentityRepository,
		UserProfile:       userProfileRepository,
		ProjectPreference: projectPreferenceRepository,
	}

	APP = &App{
		Service:    services,
		Repository: repositories,
	}
}
