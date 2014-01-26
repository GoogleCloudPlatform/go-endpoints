package endpoints

import "net/http"

//An user api's error
type ApiError struct {
	Name string
	Msg  string
	Code int
}

//ApiError is an error
func (a *ApiError) Error() string {
	return a.Msg
}

//Create a new ApiError for custom error
func NewApiError(name string, msg string, code int) error {
	return &ApiError{Name: name, Msg: msg, Code: code}
}

//Create a new ApiError as Internal Server Error (status code:500)
func NewInternalServerError(msg string) error {
	return NewApiError("Internal Server Error", msg, http.StatusInternalServerError)
}

//Create a new ApiError as Bad Request (status code:400)
func NewBadRequestError(msg string) error {
	return NewApiError("Bad Request", msg, http.StatusBadRequest)
}

//Create a new ApiError as Unauthorized (status code:401)
func NewUnauthorizedError(msg string) error {
	return NewApiError("Unauthorized", msg, http.StatusUnauthorized)
}

//Create a new ApiError as Not Found (status code:404)
func NewNotFoundError(msg string) error {
	return NewApiError("Not Found", msg, http.StatusNotFound)
}

//Create a new ApiError as Forbidden (status code:403)
func NewForbidden(msg string) error {
	return NewApiError("Forbidden", msg, http.StatusForbidden)
}

//Create a new ApiError as Conflict (status code:409)
func NewConflictError(msg string) error {
	return NewApiError("Conflict", msg, http.StatusConflict)
}

var (
	InternalServerError = NewInternalServerError("Internal Server Error")
	BadRequestError     = NewBadRequestError("Bad Request")
	UnauthorizedError   = NewUnauthorizedError("Unauthorized")
	ForbiddenError      = NewForbidden("Forbidden")
	NotFoundError       = NewNotFoundError("Not Found")
	ConflictError       = NewConflictError("Conflict")
)
