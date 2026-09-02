package httpapi

import "context"

type ctxKey int

const (
	requestIDKey ctxKey = iota
	usernameCtxKey
)

func withRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

func withUsername(ctx context.Context, username string) context.Context {
	return context.WithValue(ctx, usernameCtxKey, username)
}
