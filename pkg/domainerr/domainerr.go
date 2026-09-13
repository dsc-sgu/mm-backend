// Package domainerr abstracts business errors from web framework
// or transport representation.
//
// Conversion to the transport level error codes happens
// at the boundary, see pkg/middleware/errors.go
package domainerr

// Kind is a classification of a business error.
type Kind int

const (
	KindUnknown Kind = iota
	KindNotFound
	KindForbidden
	KindUnauthorized
	KindConflict
	KindBadRequest
	KindLocked
	KindGone
)

type Error struct {
	kind    Kind
	message string
}

func New(kind Kind, message string) *Error {
	return &Error{kind: kind, message: message}
}

func (e *Error) Error() string { return e.message }

func (e *Error) Kind() Kind { return e.kind }
