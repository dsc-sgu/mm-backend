package middleware

import (
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"go.uber.org/zap"

	"github.com/dsc-sgu/mm-backend/pkg/domainerr"
)

// Translates business errors to HTTP status codes
func toHttpStatus(k domainerr.Kind) int {
	switch k {
	case domainerr.KindNotFound:
		return http.StatusNotFound
	case domainerr.KindForbidden:
		return http.StatusForbidden
	case domainerr.KindUnauthorized:
		return http.StatusUnauthorized
	case domainerr.KindConflict:
		return http.StatusConflict
	case domainerr.KindBadRequest:
		return http.StatusBadRequest
	case domainerr.KindLocked:
		return http.StatusLocked
	case domainerr.KindGone:
		return http.StatusGone
	default:
		return http.StatusInternalServerError
	}
}

// Overrides huma's error construction so that:
//   - domainerr.Error values (business errors) are mapped to the right HTTP
//     status
//   - unexpected errors (anything else) are logged in full and
//     hidden from the client behind a generic message, instead of huma's
//     default of echoing err.Error() straight into the response body.
func InstallErrorHandling() {
	defaultNewError := huma.NewError
	huma.NewError = func(status int, msg string, errs ...error) huma.StatusError {
		for _, e := range errs {
			var domainErr *domainerr.Error
			if errors.As(e, &domainErr) {
				return defaultNewError(toHttpStatus(domainErr.Kind()), domainErr.Error())
			}
		}

		if status >= http.StatusInternalServerError {
			zap.L().Error("unexpected error", zap.Int("status", status), zap.Errors("errors", errs))
			return defaultNewError(status, "internal server error")
		}
		return defaultNewError(status, msg, errs...)
	}
}
