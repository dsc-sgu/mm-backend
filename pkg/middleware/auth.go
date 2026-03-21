package middleware

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/google/uuid"

	"github.com/dsc-sgu/mm-backend/internal/auth/session"
)

// AuthMiddleware validates the session cookie and injects session data into
// the request context. Apply to protected route groups only.
func AuthMiddleware(sessionRepo session.Repo) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		r, w := humago.Unwrap(ctx)

		cookie, err := r.Cookie(session.CookieName)
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		sessionID, err := uuid.Parse(cookie.Value)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		s, err := sessionRepo.GetByID(sessionID)
		if err != nil || s == nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		goCtx := session.WithSessionID(r.Context(), sessionID)
		goCtx = session.WithUserID(goCtx, s.UserID)
		next(humago.NewContext(ctx.Operation(), r.WithContext(goCtx), w))
	}
}

// FakeAuthMiddleware injects a user ID from the fake_user_id query parameter.
// Used for local development only (EnableAuth=false).
func FakeAuthMiddleware() func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		r, w := humago.Unwrap(ctx)

		var userID uuid.UUID
		if fakeID := r.URL.Query().Get("fake_user_id"); fakeID != "" {
			parsed, err := uuid.Parse(fakeID)
			if err == nil {
				userID = parsed
			}
		}

		goCtx := session.WithUserID(r.Context(), userID)
		next(humago.NewContext(ctx.Operation(), r.WithContext(goCtx), w))
	}
}
