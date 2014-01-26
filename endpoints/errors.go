package endpoints

import (
	"encoding/json"
	"net/http"
	"strings"
)

// An user api's error
type ApiError struct {
	Name string
	Msg  string
	Code int
}

// ApiError is an error
func (a *ApiError) Error() string {
	return a.Msg
}

// Create a new ApiError for custom error
func NewApiError(name string, msg string, code int) error {
	return &ApiError{Name: name, Msg: msg, Code: code}
}

// Create a new ApiError as Internal Server Error (status code:500)
func NewInternalServerError(msg string) error {
	return NewApiError("Internal Server Error", msg, http.StatusInternalServerError)
}

// Create a new ApiError as Bad Request (status code:400)
func NewBadRequestError(msg string) error {
	return NewApiError("Bad Request", msg, http.StatusBadRequest)
}

// Create a new ApiError as Unauthorized (status code:401)
func NewUnauthorizedError(msg string) error {
	return NewApiError("Unauthorized", msg, http.StatusUnauthorized)
}

// Create a new ApiError as Not Found (status code:404)
func NewNotFoundError(msg string) error {
	return NewApiError("Not Found", msg, http.StatusNotFound)
}

// Create a new ApiError as Forbidden (status code:403)
func NewForbiddenError(msg string) error {
	return NewApiError("Forbidden", msg, http.StatusForbidden)
}

// Create a new ApiError as Conflict (status code:409)
func NewConflictError(msg string) error {
	return NewApiError("Conflict", msg, http.StatusConflict)
}

var (
	InternalServerError = NewInternalServerError("Internal Server Error")
	BadRequestError     = NewBadRequestError("Bad Request")
	UnauthorizedError   = NewUnauthorizedError("Unauthorized")
	ForbiddenError      = NewForbiddenError("Forbidden")
	NotFoundError       = NewNotFoundError("Not Found")
	ConflictError       = NewConflictError("Conflict")
)

// errorResponse is SPI-compatible error response
type errorResponse struct {
	// Currently always "APPLICATION_ERROR"
	State string `json:"state"`
	Name  string `json:"error_name"`
	Msg   string `json:"error_message,omitempty"`
	Code  int    `json:"-"`
}

// errorNames is a slice of special error names (or better, their prefixes).
// First element is default error name.
// See newErrorResponse method for details.
var errorNames = []string{
	"Internal Server Error",
	"Bad Request",
	"Unauthorized",
	"Forbidden",
	"Not Found",
}

// errorCodes is a slice of special error codes (or better, their prefixes).
// See newErrorResponse method for details.
var errorCodes = []int{
	http.StatusInternalServerError,
	http.StatusBadRequest,
	http.StatusUnauthorized,
	http.StatusForbidden,
	http.StatusNotFound,
}

// Creates and initializes a new errorResponse.
// If msg contains any of errorNames then errorResponse.Name will be set
// to that name and the rest of the msg becomes errorResponse.Msg.
// Otherwise, a default error name is used and msg argument
// is errorResponse.Msg.
func newErrorResponse(e error) *errorResponse {
	switch t := e.(type) {
	case *ApiError:
		return &errorResponse{State: "APPLICATION_ERROR", Name: t.Name, Msg: t.Msg, Code: t.Code}
	}

	msg := e.Error()

	err := &errorResponse{State: "APPLICATION_ERROR"}
	for i, name := range errorNames {
		if strings.HasPrefix(msg, name) {
			err.Name = name
			err.Msg = msg[len(name):]
			err.Code = errorCodes[i]
		}
	}
	if err.Name == "" {
		err.Name = errorNames[0]
		err.Msg = msg
		//for compatibility, Before behavior, always return 400 HTTP Status Code.
		err.Code = 400
	}
	return err
}

// writeError writes SPI-compatible error response.
func writeError(w http.ResponseWriter, err error) {
	errResp := newErrorResponse(err)
	w.WriteHeader(errResp.Code)
	json.NewEncoder(w).Encode(errResp)
}
