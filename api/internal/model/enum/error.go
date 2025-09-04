package enum

import "errors"

var (
	ErrSubscriptionExpired error = errors.New("subscription expired")
	ErrQuotaExceeded       error = errors.New("quota exceeded")
)
