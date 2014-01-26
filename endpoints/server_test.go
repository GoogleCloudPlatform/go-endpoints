package endpoints

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	tu "github.com/crhym3/aegot/testutils"
)

type msg struct {
	Name string `json:"name"`
}

type ServerTestService struct{}

func (s *ServerTestService) Void(r *http.Request, _, _ *VoidMessage) error {
	return nil
}

func (s *ServerTestService) Error(r *http.Request, _, _ *VoidMessage) error {
	return errors.New("Dummy error")
}

func (s *ServerTestService) Msg(r *http.Request, req, resp *msg) error {
	resp.Name = req.Name
	return nil
}

func (s *ServerTestService) CustomApiError(r *http.Request, req, resp *msg) error {
	return NewApiError("MethodNotAllowed", "MethodNotAllowed", http.StatusMethodNotAllowed)
}

func (s *ServerTestService) InternalServer(r *http.Request, req, resp *msg) error {
	return InternalServerError
}

func (s *ServerTestService) BadRequest(r *http.Request, req, resp *msg) error {
	return BadRequestError
}

func (s *ServerTestService) NotFound(r *http.Request, req, resp *msg) error {
	return NotFoundError
}

func (s *ServerTestService) Forbidden(r *http.Request, req, resp *msg) error {
	return ForbiddenError
}

func (s *ServerTestService) Unauthorized(r *http.Request, req, resp *msg) error {
	return UnauthorizedError
}

func (s *ServerTestService) Conflict(r *http.Request, req, resp *msg) error {
	return ConflictError
}

func TestServerServeHTTP(t *testing.T) {
	myService := &ServerTestService{}
	rpc := &RpcService{
		name:     "ServerTestService",
		rcvr:     reflect.ValueOf(myService),
		rcvrType: reflect.TypeOf(myService),
		methods:  make(map[string]*ServiceMethod),
	}
	for i := 0; i < rpc.rcvrType.NumMethod(); i++ {
		meth := rpc.rcvrType.Method(i)
		rpc.methods[meth.Name] = &ServiceMethod{
			method:   &meth,
			ReqType:  meth.Type.In(2).Elem(),
			RespType: meth.Type.In(3).Elem(),
		}
	}

	srvMap := &serviceMap{services: make(map[string]*RpcService)}
	srvMap.services[rpc.name] = rpc
	server := &Server{root: "/_ah/spi", services: srvMap}

	tts := []struct {
		httpVerb           string
		srvMethod, in, out string
		code               int
	}{

		{"POST", "Void", `{}`, `{}`, http.StatusOK},
		{"POST", "Msg", `{"name":"alex"}`, `{"name":"alex"}`, http.StatusOK},

		{"POST", "Error", `{}`, ``, http.StatusBadRequest},
		{"POST", "Msg", ``, ``, http.StatusBadRequest},
		{"POST", "DoesNotExist", `{}`, ``, http.StatusBadRequest},

		{"POST", "InternalServer", `{}`, ``, http.StatusInternalServerError},
		{"POST", "BadRequest", `{}`, ``, http.StatusBadRequest},
		{"POST", "NotFound", `{}`, ``, http.StatusNotFound},
		{"POST", "Forbidden", `{}`, ``, http.StatusForbidden},
		{"POST", "Unauthorized", `{}`, ``, http.StatusUnauthorized},
		{"POST", "CustomApiError", `{}`, ``, http.StatusMethodNotAllowed},

		{"GET", "Void", `{}`, ``, http.StatusBadRequest},
		{"PUT", "Void", `{}`, ``, http.StatusBadRequest},
		{"HEAD", "Void", `{}`, ``, http.StatusBadRequest},
		{"DELETE", "Void", `{}`, ``, http.StatusBadRequest},
	}

	for i, tt := range tts {
		path := "/" + rpc.name + "." + tt.srvMethod
		r, delAppengineCtx := tu.NewTestRequest(tt.httpVerb, path, []byte(tt.in))
		defer delAppengineCtx()
		w := httptest.NewRecorder()

		// make sure endpoints.NewContext is called at least once
		// and verify it's not nil, just in case.
		if c := NewContext(r); c == nil {
			t.Errorf("%d: got nil context!", i)
			continue
		}

		// do the fake request
		server.ServeHTTP(w, r)

		// verify endpoints.context has been destroyed
		if c, exists := ctxs[r]; exists {
			t.Errorf("%d: expected context to be deleted: %#v", i, c)
		}

		// make sure the response is correct
		out := strings.TrimSpace(w.Body.String())
		if tt.code == http.StatusOK && out != tt.out {
			t.Errorf("%d: expected %q, got %q", i, tt.out, out)
		}
		if w.Code != tt.code {
			t.Errorf("%d: expected status code %d, got %d",
				i, tt.code, w.Code)
		}
	}
}
