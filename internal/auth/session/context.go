package session

import (
	"context"

	"github.com/google/uuid"
)

type (
	userIDKeyType    struct{}
	sessionIDKeyType struct{}
)

var (
	userIDKey    = userIDKeyType{}
	sessionIDKey = sessionIDKeyType{}
)

func WithUserID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, userIDKey, id)
}

func UserIDFromContext(ctx context.Context) uuid.UUID {
	v := ctx.Value(userIDKey)
	id, _ := v.(uuid.UUID)
	return id
}

func WithSessionID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, sessionIDKey, id)
}

func SessionIDFromContext(ctx context.Context) uuid.UUID {
	v := ctx.Value(sessionIDKey)
	id, _ := v.(uuid.UUID)
	return id
}
