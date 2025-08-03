package repository

type NotificationRepository interface {
	APIKeyReader
	APIKeyWriter
}

type NotificationReader interface {
}

type NotificationWriter interface {
}
