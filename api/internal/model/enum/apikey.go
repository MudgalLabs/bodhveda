package enum

type APIKeyScope string

const (
	// APIKeyScopeFull has all the permissions of the API key.
	//
	// It can create, read, update, and delete *ALL* resources.
	// Only this scope *CAN* send notifications.
	APIKeyScopeFull APIKeyScope = "full"
	// APIKeyScopeRecipient has limited permissions.
	//
	// It can do *ALL recipient* operations, like fetch notifications,
	// mark a notification as read, delete a notification, preferences,
	// mutes, subs, and more.
	//
	// This scope *CANNOT* send notifications.
	//
	// It is recommended that the `recipient` are *NOT* easy to guess,
	// otherwise, anyone can use this API key to perform actions
	// on behalf of the user.
	APIKeyScopeRecipient APIKeyScope = "recipient"
)
