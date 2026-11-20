package logger

import (
	"net/http"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func Init(logLevel zap.AtomicLevel) {
	conf := zap.NewDevelopmentConfig()
	conf.Level = logLevel

	zap.ReplaceGlobals(zap.Must(conf.Build()))
}

// statusRecorder wraps http.ResponseWriter to remember the status code
// written by the handlers further down the chain, so it can be logged
// after the fact.
type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (r *statusRecorder) WriteHeader(status int) {
	if r.wroteHeader {
		return
	}
	r.status = status
	r.wroteHeader = true
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if !r.wroteHeader {
		// net/http defaults to 200 if the handler writes a body without
		// calling WriteHeader explicitly; mirror that here.
		r.WriteHeader(http.StatusOK)
	}
	return r.ResponseWriter.Write(b)
}

func ZapMiddleware() func(http.Handler) http.Handler {
	// Building zap logger without stacktrace for error logging:
	// This middleware only ever sees the status code, never the underlying error,
	// so a caller/stacktrace would just point at this call site every time
	//
	// This logger builds only once.
	outcomeLogger := zap.L().WithOptions(
		zap.WithCaller(false),
		zap.AddStacktrace(zapcore.InvalidLevel),
	)

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

			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)

			logFn := outcomeLogger.Info
			if rec.status >= http.StatusInternalServerError {
				logFn = outcomeLogger.Error
			}
			logFn(
				"Request finished",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", rec.status),
			)
		})
	}
}
