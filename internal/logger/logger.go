package logger

import (
	"net/http"

	"go.uber.org/zap"
)

func Init(logLevel zap.AtomicLevel) {
	conf := zap.NewDevelopmentConfig()
	conf.Level = logLevel

	zap.ReplaceGlobals(zap.Must(conf.Build()))
}

func ZapMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			zap.L().Info(
				"Request received",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.String("query", r.URL.RawQuery),
				zap.String("ip", r.Host),
				zap.String("user-agent", r.UserAgent()),
			)
			next.ServeHTTP(w, r)
		})
	}
}
