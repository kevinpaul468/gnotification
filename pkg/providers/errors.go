package providers

import "errors"

var (
	ErrProviderNotFound = errors.New("provider not found")
	ErrInvalidConfig    = errors.New("invalid provider configuration")
	ErrSendFailed       = errors.New("failed to send notification")
	ErrHealthCheckFailed = errors.New("health check failed")
)
