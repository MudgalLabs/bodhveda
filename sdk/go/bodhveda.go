// Package bodhveda provides a client SDK for the Bodhveda API.
package bodhveda

import (
	"context"
	"fmt"
	"net/url"

	"github.com/MudgalLabs/bodhveda/sdk/go/internal/httpx"
	"github.com/MudgalLabs/bodhveda/sdk/go/internal/routes"
)

// Client is used for interacting with the Bodhveda API.
type Client struct {
	client *httpx.Client

	Notifications *Notifications
	Recipients    *Recipients
}

// ClientOptions configures the Bodhveda client.
type ClientOptions struct {
	APIURL string
	Debug  bool
}

// NewClient creates a new Bodhveda client.
func NewClient(apiKey string, opts *ClientOptions) *Client {
	baseURL := "https://api.bodhveda.com"
	debug := false

	if opts != nil {
		if opts.APIURL != "" {
			baseURL = opts.APIURL
		}

		debug = opts.Debug
	}

	client := httpx.NewClient(apiKey, baseURL, debug)

	bodhveda := &Client{
		client:        client,
		Notifications: &Notifications{client},
		Recipients: &Recipients{
			client:        client,
			Notifications: &RecipientsNotifications{client: client},
			Preferences:   &RecipientsPreferences{client: client},
		},
	}

	return bodhveda
}

// NotificationsService defines notification-related API methods.
type NotificationsService interface {
	// Send sends a notification.
	Send(ctx context.Context, req *SendNotificationRequest) (*SendNotificationResponse, error)
}

// Notifications implements NotificationsService.
type Notifications struct {
	client *httpx.Client
}

func (notifications *Notifications) Send(ctx context.Context, req *SendNotificationRequest) (*SendNotificationResponse, error) {
	var resp SendNotificationResponse
	err := notifications.client.Do(ctx, "POST", routes.NotificationsSend, req, &resp)
	return &resp, err
}

// RecipientService defines recipient-related API methods.
type RecipientService interface {
	// Create creates a new recipient.
	Create(ctx context.Context, req *CreateRecipientRequest) (*CreateRecipientResponse, error)

	// CreateBatch creates multiple recipients in a batch.
	CreateBatch(ctx context.Context, req *CreateRecipientsBatchRequest) (*CreateRecipientsBatchResponse, error)

	// Get retrieves a recipient by ID.
	Get(ctx context.Context, recipientID string) (*GetRecipientResponse, error)

	// Update updates a recipient by ID.
	Update(ctx context.Context, recipientID string, req *UpdateRecipientRequest) (*UpdateRecipientResponse, error)

	// Delete deletes a recipient by ID.
	Delete(ctx context.Context, recipientID string) error
}

// Recipients implements RecipientService.
type Recipients struct {
	client *httpx.Client

	Notifications *RecipientsNotifications
	Preferences   *RecipientsPreferences
}

func (recipients *Recipients) Create(ctx context.Context, req *CreateRecipientRequest) (*CreateRecipientResponse, error) {
	var resp CreateRecipientResponse
	err := recipients.client.Do(ctx, "POST", routes.RecipientsCreate, req, &resp)
	return &resp, err
}

func (recipients *Recipients) CreateBatch(ctx context.Context, req *CreateRecipientsBatchRequest) (*CreateRecipientsBatchResponse, error) {
	var resp CreateRecipientsBatchResponse
	err := recipients.client.Do(ctx, "POST", routes.RecipientsCreateBatch, req, &resp)
	return &resp, err
}

func (recipients *Recipients) Get(ctx context.Context, recipientID string) (*GetRecipientResponse, error) {
	var resp GetRecipientResponse
	err := recipients.client.Do(ctx, "GET", routes.RecipeientsGet(recipientID), nil, &resp)
	return &resp, err
}

func (recipients *Recipients) Update(ctx context.Context, recipientID string, req *UpdateRecipientRequest) (*UpdateRecipientResponse, error) {
	var resp UpdateRecipientResponse
	err := recipients.client.Do(ctx, "PATCH", routes.RecipeientsUpdate(recipientID), req, &resp)
	return &resp, err
}

func (recipients *Recipients) Delete(ctx context.Context, recipientID string) error {
	return recipients.client.Do(ctx, "DELETE", routes.RecipeientsDelete(recipientID), nil, nil)
}

// ReciepientsNotificationsService provides notification methods for a recipient.
type ReciepientsNotificationsService interface {
	// List lists notifications for a recipient.
	List(ctx context.Context, recipientID string, req *ListNotificationsRequest) (*ListNotificationsResponse, error)

	// UnreadCount gets the count of unread notifications for a recipient.
	UnreadCount(ctx context.Context, recipientID string) (*UnreadCountResponse, error)

	// UpdateState updates the state of one or more notifications for a recipient.
	UpdateState(ctx context.Context, recipientID string, req *UpdateNotificationsStateRequest) (*UpdateNotificationsStateResponse, error)

	// Delete deletes one or more notifications for a recipient.
	Delete(ctx context.Context, recipientID string, req *DeleteNotificationsRequest) (*DeleteNotificationsResponse, error)
}

// RecipientsNotifications implements ReciepientsNotificationsService.
type RecipientsNotifications struct {
	client *httpx.Client
}

func (recipientsNotifications *RecipientsNotifications) List(ctx context.Context, recipientID string, req *ListNotificationsRequest) (*ListNotificationsResponse, error) {
	params := url.Values{}

	if req != nil {
		if req.Limit != nil && *req.Limit > 0 {
			params.Set("limit", fmt.Sprintf("%d", *req.Limit))
		}

		if req.Before != nil && *req.Before != "" {
			params.Set("before", *req.Before)
		}

		if req.After != nil && *req.After != "" {
			params.Set("after", *req.After)
		}
	}

	path := routes.RecipientsNotificationsList(recipientID)
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	var resp ListNotificationsResponse
	err := recipientsNotifications.client.Do(ctx, "GET", path, nil, &resp)
	return &resp, err
}

func (recipientsNotifications *RecipientsNotifications) UnreadCount(ctx context.Context, recipientID string) (*UnreadCountResponse, error) {
	var resp UnreadCountResponse
	err := recipientsNotifications.client.Do(ctx, "GET", routes.RecipientsNotificationUnreadCount(recipientID), nil, &resp)
	return &resp, err
}

func (recipientsNotifications *RecipientsNotifications) UpdateState(ctx context.Context, recipientID string, req *UpdateNotificationsStateRequest) (*UpdateNotificationsStateResponse, error) {
	var resp UpdateNotificationsStateResponse
	err := recipientsNotifications.client.Do(ctx, "PATCH", routes.RecipientsNotificationsUpdateState(recipientID), req, &resp)
	return &resp, err
}

func (recipientsNotifications *RecipientsNotifications) Delete(ctx context.Context, recipientID string, req *DeleteNotificationsRequest) (*DeleteNotificationsResponse, error) {
	var resp DeleteNotificationsResponse
	err := recipientsNotifications.client.Do(ctx, "DELETE", routes.RecipientsNotificationsDelete(recipientID), req, &resp)
	return &resp, err
}

// RecipientPreferencesService provides preference methods for a recipient.
type RecipientPreferencesService interface {
	// List lists preferences for a recipient.
	List(ctx context.Context, recipientID string) (*ListPreferencesResponse, error)

	// Set sets a preference for a recipient.
	Set(ctx context.Context, recipientID string, req *SetPreferenceRequest) (*SetPreferenceResponse, error)

	// Check checks a preference for a recipient.
	Check(ctx context.Context, recipientID string, req *CheckPreferenceRequest) (*CheckPreferenceResponse, error)
}

// RecipientsPreferences implements RecipientPreferencesService.
type RecipientsPreferences struct {
	client *httpx.Client
}

func (recipientsPreferences *RecipientsPreferences) List(ctx context.Context, recipientID string) (*ListPreferencesResponse, error) {
	var resp ListPreferencesResponse
	err := recipientsPreferences.client.Do(ctx, "GET", routes.RecipientsPreferencesList(recipientID), nil, &resp)
	return &resp, err
}

func (recipientsPreferences *RecipientsPreferences) Set(ctx context.Context, recipientID string, req *SetPreferenceRequest) (*SetPreferenceResponse, error) {
	var resp SetPreferenceResponse
	err := recipientsPreferences.client.Do(ctx, "PATCH", routes.RecipientsPreferencesSet(recipientID), req, &resp)
	return &resp, err
}

func (recipientsPreferences *RecipientsPreferences) Check(ctx context.Context, recipientID string, req *CheckPreferenceRequest) (*CheckPreferenceResponse, error) {
	params := url.Values{}

	if req != nil {
		params.Set("channel", req.Target.Channel)
		params.Set("topic", req.Target.Topic)
		params.Set("event", req.Target.Event)
	}

	path := routes.RecipientsPreferencesCheck(recipientID)
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	var resp CheckPreferenceResponse
	err := recipientsPreferences.client.Do(ctx, "GET", path, nil, &resp)
	return &resp, err
}
