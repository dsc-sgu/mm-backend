package middleware

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/dsc-sgu/mm-backend/internal/auth/session"
)

func AuthMiddleware(
	sessionRepo session.Repo,
) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(session.CookieName)
			if err != nil {
				http.Error(w, "WRONG_CREDENTIALS", http.StatusUnauthorized)
				return
			}

			sessionID, err := uuid.Parse(cookie.Value)
			if err != nil {
				http.Error(
					w,
					"INTERNAL_SERVER_ERROR",
					http.StatusInternalServerError,
				)
				return
			}

			s, err := sessionRepo.GetById(sessionID)
			if err != nil {
				http.Error(w, "WRONG_CREDENTIALS", http.StatusUnauthorized)
				return
			}

			ctx := session.WithSessionID(r.Context(), sessionID)
			ctx = session.WithUserID(ctx, s.UserId)
			r = r.WithContext(ctx)

			next.ServeHTTP(w, r)
		})
	}
}

func FakeAuthMiddleware() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u, err := uuid.Parse(r.URL.Query().Get("fakeUserID"))
			if err != nil {
				http.Error(
					w,
					"INTERNAL_SERVER_ERROR",
					http.StatusInternalServerError,
				)
				return
			}

			ctx := session.WithUserID(r.Context(), u)
			r = r.WithContext(ctx)
			next.ServeHTTP(w, r)
		})
	}
}
