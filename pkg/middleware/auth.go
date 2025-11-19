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
			sessionID, err := r.Cookie(session.CookieName)
			if err != nil {
				http.Error(w, "WRONG_CREDENTIALS", http.StatusUnauthorized)
				return
			}

			u, err := uuid.Parse(sessionID.Value)
			if err != nil {
				http.Error(
					w,
					"INTERNAL_SERVER_ERROR",
					http.StatusInternalServerError,
				)
				return
			}

			s, err := sessionRepo.GetById(u)
			if err != nil {
				http.Error(w, "WRONG_CREDENTIALS", http.StatusUnauthorized)
				return
			}

			ctx := session.WithUserID(r.Context(), s.UserId)
			r = r.WithContext(ctx)

			next.ServeHTTP(w, r)
		})
	}
}

// TODO: add optional middleware enabling to main.go
