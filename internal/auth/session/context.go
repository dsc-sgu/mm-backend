package session

import (
	"context"

	"github.com/google/uuid"
)

type contextKey struct{}

var userIDKey = contextKey{}

func WithUserID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, userIDKey, id)
}

func UserIDFromContext(ctx context.Context) uuid.UUID {
	v := ctx.Value(userIDKey)
	if id, ok := v.(uuid.UUID); ok {
		return id
	}
	return uuid.Nil
}
