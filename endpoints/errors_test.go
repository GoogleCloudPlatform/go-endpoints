package endpoints

import (
	"errors"
	"net/http"
	"reflect"
	"testing"
)

func TestCustomErrorResponse(t *testing.T) {
	tts := []*struct {
		errMsg string
		want   *errorResponse
	}{
		{"Not Found: test msg", &errorResponse{
			State: "APPLICATION_ERROR",
			Name:  "Not Found",
			Msg:   "test msg",
			Code:  http.StatusNotFound,
		}},
		{"Random error", &errorResponse{
			State: "APPLICATION_ERROR",
			Name:  "Internal Server Error",
			Msg:   "Random error",
			Code:  http.StatusBadRequest,
		}},
	}

	for i, tt := range tts {
		err := errors.New(tt.errMsg)
		res := newErrorResponse(err)
		if !reflect.DeepEqual(res, tt.want) {
			t.Errorf("%d: newErrorResponse(%q) = %#v; want %#v",
				i, tt.errMsg, res, tt.want)
		}
	}
}

func TestAPIErrorResponse(t *testing.T) {
	res := newErrorResponse(BadRequestError)
	want := &errorResponse{
		State: "APPLICATION_ERROR",
		Name:  res.Name,
		Msg:   res.Msg,
		Code:  res.Code,
	}
	if !reflect.DeepEqual(res, want) {
		t.Errorf("newErrorResponse(%#v) = %#v; want %#v",
			BadRequestError, res, want)
	}
}
