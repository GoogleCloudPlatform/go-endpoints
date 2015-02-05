// +build appengine

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
	InternalServerError = NewInternalServerError(errorNames[http.StatusInternalServerError])
	// BadRequestError is default error with http.StatusBadRequest (400)
	BadRequestError = NewBadRequestError(errorNames[http.StatusBadRequest])
	// UnauthorizedError is default error with http.StatusUnauthorized (401)
	UnauthorizedError = NewUnauthorizedError(errorNames[http.StatusUnauthorized])
	// ForbiddenError is default error with http.StatusForbidden (403)
	ForbiddenError = NewForbiddenError(errorNames[http.StatusForbidden])
	// NotFoundError is default error with http.StatusNotFound (404)
	NotFoundError = NewNotFoundError(errorNames[http.StatusNotFound])
	// ConflictError is default error with http.StatusConflict (409)
	ConflictError = NewConflictError(errorNames[http.StatusConflict])

	// errorNames is a map of known error names (or better, their prefixes).
	// See newErrorResponse method for details.
	errorNames = map[int]string{
		http.StatusInternalServerError: "Internal Server Error",
		http.StatusBadRequest:          "Bad Request",
		http.StatusUnauthorized:        "Unauthorized",
		http.StatusForbidden:           "Forbidden",
		http.StatusNotFound:            "Not Found",
		http.StatusConflict:            "Conflict",
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
	return &APIError{Name: name, Msg: msg, Code: code}
}

// Errorf creates a new APIError for custom error given a format string and arguments.
func Errorf(code int, name string, format string, args ...interface{}) error {
	return &APIError{Name: name, Msg: fmt.Sprintf(format, args...), Code: code}
}

// NewInternalServerError creates a new APIError with Internal Server Error status (500)
func NewInternalServerError(format string, args ...interface{}) error {
	return Errorf(http.StatusInternalServerError, "Internal Server Error", format, args...)
}

// NewBadRequestError creates a new APIError with Bad Request status (400)
func NewBadRequestError(format string, args ...interface{}) error {
	return Errorf(http.StatusBadRequest, "Bad Request", format, args...)
}

// NewUnauthorizedError creates a new APIError with Unauthorized status (401)
func NewUnauthorizedError(format string, args ...interface{}) error {
	return Errorf(http.StatusUnauthorized, "Unauthorized", format, args...)
}

// NewNotFoundError creates a new APIError with Not Found status (404)
func NewNotFoundError(format string, args ...interface{}) error {
	return Errorf(http.StatusNotFound, "Not Found", format, args...)
}

// NewForbiddenError creates a new APIError with Forbidden status (403)
func NewForbiddenError(format string, args ...interface{}) error {
	return Errorf(http.StatusForbidden, "Forbidden", format, args...)
}

// NewConflictError creates a new APIError with Conflict status (409)
func NewConflictError(format string, args ...interface{}) error {
	return Errorf(http.StatusConflict, "Conflict", format, args...)
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
// If msg contains any of errorNames then errorResponse.Name will be set
// to that name and the rest of the msg becomes errorResponse.Msg.
// Otherwise, a default error name is used and msg argument
// is errorResponse.Msg.
func newErrorResponse(err error) *errorResponse {
	if apiErr, ok := err.(*APIError); ok {
		return &errorResponse{State: "APPLICATION_ERROR", Name: apiErr.Name, Msg: apiErr.Msg, Code: apiErr.Code}
	}

	msg := err.Error()
	for code, name := range errorNames {
		if strings.HasPrefix(msg, name) {
			return &errorResponse{
				State: "APPLICATION_ERROR",
				Name:  name,
				Msg:   strings.Trim(msg[len(name):], " :"),
				Code:  code,
			}
		}
	}

	return &errorResponse{
		State: "APPLICATION_ERROR",
		Name:  errorNames[http.StatusInternalServerError],
		Msg:   msg,
		//for compatibility, Before behavior, always return 400 HTTP Status Code.
		// TODO(alex): where is 400 coming from?
		Code: http.StatusBadRequest,
	}
}

// writeError writes SPI-compatible error response.
func writeError(w http.ResponseWriter, err error) {
	errResp := newErrorResponse(err)
	w.WriteHeader(errResp.Code)
	json.NewEncoder(w).Encode(errResp)
}
