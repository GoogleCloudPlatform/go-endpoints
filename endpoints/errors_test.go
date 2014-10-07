package endpoints

import (
	"errors"
	"net/http"
	"testing"
)

func TestCustomErrorResponse(t *testing.T) {
	msg := "test msg"
	err := errors.New("Not Found: " + msg)
	res := newErrorResponse(err)
	if s := "APPLICATION_ERROR"; res.State != s {
		t.Errorf("have state %q, want %q", res.State, s)
	}
	if n := "Not Found"; res.Name != n {
		t.Errorf("have name %q, want %q", res.Name, n)
	}
	if res.Msg != msg {
		t.Errorf("have message %q, want %q", res.Msg, msg)
	}
	if res.Code != http.StatusNotFound {
		t.Errorf("have code %d, want %d", res.Code, http.StatusNotFound)
	}

	err = errors.New("Random error")
	res = newErrorResponse(err)
	if res.Msg != err.Error() {
		t.Errorf("have custom error msg %q, want %q", res.Msg, err.Error())
	}
}

func TestApiErrorResponse(t *testing.T) {
	res := newErrorResponse(BadRequestError)
	if s := "APPLICATION_ERROR"; res.State != s {
		t.Errorf("have state %q, want %q", res.State, s)
	}
	if n := "Bad Request"; res.Name != n {
		t.Errorf("have name %q, want %q", res.Name, n)
	}
	if m := "Bad Request"; res.Msg != m {
		t.Errorf("have message %q, want %q", res.Msg, m)
	}
	if res.Code != http.StatusBadRequest {
		t.Errorf("have code %d, want %d", res.Code, http.StatusBadRequest)
	}
}
