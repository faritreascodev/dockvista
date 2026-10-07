package httpapi

import (
	"context"

	"dockvista/internal/core/domain"
)

type ctxKey int

const (
	requestIDKey ctxKey = iota
	usernameCtxKey
	principalCtxKey
)

func withRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

func withUsername(ctx context.Context, username string) context.Context {
	return context.WithValue(ctx, usernameCtxKey, username)
}

func withPrincipal(ctx context.Context, p domain.Principal) context.Context {
	ctx = context.WithValue(ctx, principalCtxKey, p)
	return withUsername(ctx, p.Username)
}

func principalFrom(ctx context.Context) domain.Principal {
	p, _ := ctx.Value(principalCtxKey).(domain.Principal)
	return p
}
