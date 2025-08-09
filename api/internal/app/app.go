package app

import (
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudgallabs/bodhveda/internal/env"
	"github.com/mudgallabs/bodhveda/internal/feature/user_identity"
	"github.com/mudgallabs/bodhveda/internal/feature/user_profile"
	jobs "github.com/mudgallabs/bodhveda/internal/job"
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
	DB          *pgxpool.Pool
	AsynqClient *asynq.Client

	Service    services
	Repository repositories
}

// All the services.
type services struct {
	APIKey       *service.APIKeyService
	Notification *service.NotificationService
	Preference   *service.PreferenceService
	Project      *service.ProjectService
	Recipient    *service.RecipientService

	UserIdentity *user_identity.Service
	UserProfile  *user_profile.Service
}

// Access to all repositories for reading.
// Write access only available to services.
type repositories struct {
	APIKey         repository.APIKeyRepository
	Broadcast      repository.BroadcastRepository
	BroadcastBatch repository.BroadcastBatchRepository
	Notification   repository.NotificationRepository
	Preference     repository.PreferenceRepository
	Project        repository.ProjectRepository
	Recipient      repository.RecipientRepository

	UserIdentity user_identity.ReadWriter
	UserProfile  user_profile.ReadWriter
}

var ASYNQCLIENT *asynq.Client

func Init() {
	env.Init("../.env")

	logger.Init(env.LogLevel, env.LogFile)

	// IDK what this does but it was on the blogpost so I'm using it.
	// I think it has something to do with Go sync for multi threading?
	defer logger.Get().Sync()

	session.Init()

	db, err := dbx.Init(env.DBURL)
	if err != nil {
		logger.Get().Errorf("failed to connect to database: %v", err)
		panic(err)
	}
	DB = db

	ASYNQCLIENT, err = jobs.NewAsynqClient()
	if err != nil {
		logger.Get().Errorf("failed to create Asynq client: %v", err)
		panic(err)
	}

	oauth.InitGoogle(env.GOOGLE_CLIENT_ID, env.GOOGLE_CLIENT_SECRET, env.GOOGLE_REDIRECT_URL)

	apikeyRepository := pg.NewAPIKeyRepo(db)
	broadcastRepository := pg.NewBroadcastRepo(db)
	broadcastBatchRepository := pg.NewBroadcastBatchRepo(db)
	notificationRepository := pg.NewNotificationRepo(db)
	preferenceRepository := pg.NewPreferenceRepo(db)
	projectRepository := pg.NewProjectRepo(db)
	recipientRepository := pg.NewRecipientRepo(db)
	userProfileRepository := user_profile.NewRepository(db)
	userIdentityRepository := user_identity.NewRepository(db)

	apikeyService := service.NewAPIKeyService(apikeyRepository, projectRepository)
	preferenceService := service.NewProjectPreferenceService(preferenceRepository, recipientRepository)
	projectService := service.NewProjectService(projectRepository)
	recipientService := service.NewRecipientService(recipientRepository, ASYNQCLIENT)
	notificationService := service.NewNotificationService(notificationRepository, recipientRepository,
		preferenceRepository, broadcastRepository, broadcastBatchRepository, recipientService, ASYNQCLIENT)
	userIdentityService := user_identity.NewService(userIdentityRepository, userProfileRepository)
	userProfileService := user_profile.NewService(userProfileRepository)

	services := services{
		APIKey:       apikeyService,
		Notification: notificationService,
		Preference:   preferenceService,
		Project:      projectService,
		Recipient:    recipientService,
		UserIdentity: userIdentityService,
		UserProfile:  userProfileService,
	}

	repositories := repositories{
		APIKey:         apikeyRepository,
		Broadcast:      broadcastRepository,
		BroadcastBatch: broadcastBatchRepository,
		Notification:   notificationRepository,
		Preference:     preferenceRepository,
		Project:        projectRepository,
		Recipient:      recipientRepository,
		UserIdentity:   userIdentityRepository,
		UserProfile:    userProfileRepository,
	}

	APP = &App{
		DB:          db,
		AsynqClient: ASYNQCLIENT,
		Service:     services,
		Repository:  repositories,
	}

	// err = recipientService.CreateRandomRecipients(context.Background(), 1, 10000)
	// if err != nil {
	// 	logger.Get().Errorf("failed to create random recipients: %v", err)
	// } else {
	// 	logger.Get().Infof("created random recipients successfully")
	// }
}

func Close() {
	if DB != nil {
		DB.Close()
	}

	if ASYNQCLIENT != nil {
		err := ASYNQCLIENT.Close()
		if err != nil {
			logger.Get().Errorf("failed to close Asynq client: %v", err)
		}
	}
}
