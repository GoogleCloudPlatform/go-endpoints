package endpoints

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

var (
	// Pre-defined API errors.
	// Use NewAPIError() method to create your own.

	// InternalServerError is default error with http.StatusInternalServerError (500)
	InternalServerError = NewInternalServerError("")
	// BadRequestError is default error with http.StatusBadRequest (400)
	BadRequestError = NewBadRequestError("")
	// UnauthorizedError is default error with http.StatusUnauthorized (401)
	UnauthorizedError = NewUnauthorizedError("")
	// ForbiddenError is default error with http.StatusForbidden (403)
	ForbiddenError = NewForbiddenError("")
	// NotFoundError is default error with http.StatusNotFound (404)
	NotFoundError = NewNotFoundError("")
	// ConflictError is default error with http.StatusConflict (409)
	ConflictError = NewConflictError("")

	// knownErrors is a list of all known errors.
	knownErrors = [...]int{
		http.StatusInternalServerError,
		http.StatusBadRequest,
		http.StatusUnauthorized,
		http.StatusForbidden,
		http.StatusNotFound,
		http.StatusConflict,
	}
)

// APIError is a user custom API's error
type APIError struct {
	Name string
	Msg  string
	Code int
}

// APIError is an error
func (a *APIError) Error() string {
	return a.Msg
}

// NewAPIError Create a new APIError for custom error
func NewAPIError(name string, msg string, code int) error {
	return &APIError{name, msg, code}
}

// errorf creates a new APIError given its status code, a format string and its arguments.
func errorf(code int, format string, args ...interface{}) error {
	return &APIError{http.StatusText(code), fmt.Sprintf(format, args...), code}
}

// NewInternalServerError creates a new APIError with Internal Server Error status (500)
func NewInternalServerError(format string, args ...interface{}) error {
	return errorf(http.StatusInternalServerError, format, args...)
}

// NewBadRequestError creates a new APIError with Bad Request status (400)
func NewBadRequestError(format string, args ...interface{}) error {
	return errorf(http.StatusBadRequest, format, args...)
}

// NewUnauthorizedError creates a new APIError with Unauthorized status (401)
func NewUnauthorizedError(format string, args ...interface{}) error {
	return errorf(http.StatusUnauthorized, format, args...)
}

// NewNotFoundError creates a new APIError with Not Found status (404)
func NewNotFoundError(format string, args ...interface{}) error {
	return errorf(http.StatusNotFound, format, args...)
}

// NewForbiddenError creates a new APIError with Forbidden status (403)
func NewForbiddenError(format string, args ...interface{}) error {
	return errorf(http.StatusForbidden, format, args...)
}

// NewConflictError creates a new APIError with Conflict status (409)
func NewConflictError(format string, args ...interface{}) error {
	return errorf(http.StatusConflict, format, args...)
}

// errorResponse is SPI-compatible error response
type errorResponse struct {
	// Currently always "APPLICATION_ERROR"
	State string `json:"state"`
	Name  string `json:"error_name"`
	Msg   string `json:"error_message,omitempty"`
	Code  int    `json:"-"`
}

// Creates and initializes a new errorResponse.
// If msg contains any of knownErrors then errorResponse.Name will be set
// to that name and the rest of the msg becomes errorResponse.Msg.
// Otherwise, a default error name is used and msg argument
// is errorResponse.Msg.
func newErrorResponse(err error) *errorResponse {
	if e, ok := err.(*APIError); ok {
		return &errorResponse{"APPLICATION_ERROR", e.Name, e.Msg, e.Code}
	}
	msg := err.Error()
	for _, code := range knownErrors {
		if name := http.StatusText(code); strings.HasPrefix(msg, name) {
			return &errorResponse{"APPLICATION_ERROR", name, strings.Trim(msg[len(name):], " :"), code}
		}
	}
	//for compatibility, Before behavior, always return 400 HTTP Status Code.
	// TODO(alex): where is 400 coming from?
	return &errorResponse{"APPLICATION_ERROR", http.StatusText(http.StatusInternalServerError), msg, http.StatusBadRequest}
}

// writeError writes SPI-compatible error response.
func writeError(w http.ResponseWriter, err error) {
	errResp := newErrorResponse(err)
	w.WriteHeader(errResp.Code)
	json.NewEncoder(w).Encode(errResp)
}
