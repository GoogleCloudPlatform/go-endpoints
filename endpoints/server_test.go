package endpoints

import (
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"appengine/aetest"
)

type TestMsg struct {
	Name string `json:"name"`
}

type BytesMsg struct {
	Bytes []byte
}

type ServerTestService struct{}

func (s *ServerTestService) Void(r *http.Request, _, _ *VoidMessage) error {
	return nil
}

func (s *ServerTestService) Error(r *http.Request, _, _ *VoidMessage) error {
	return errors.New("Dummy error")
}

func (s *ServerTestService) Msg(r *http.Request, req, resp *TestMsg) error {
	resp.Name = req.Name
	return nil
}

func (s *ServerTestService) CustomAPIError(r *http.Request, req, resp *TestMsg) error {
	return NewAPIError("MethodNotAllowed", "MethodNotAllowed", http.StatusMethodNotAllowed)
}

func (s *ServerTestService) InternalServer(r *http.Request, req, resp *TestMsg) error {
	return InternalServerError
}

func (s *ServerTestService) BadRequest(r *http.Request, req, resp *TestMsg) error {
	return BadRequestError
}

func (s *ServerTestService) NotFound(r *http.Request, req, resp *TestMsg) error {
	return NotFoundError
}

func (s *ServerTestService) Forbidden(r *http.Request, req, resp *TestMsg) error {
	return ForbiddenError
}

func (s *ServerTestService) Unauthorized(r *http.Request, req, resp *TestMsg) error {
	return UnauthorizedError
}

func (s *ServerTestService) Conflict(r *http.Request, req, resp *TestMsg) error {
	return ConflictError
}

// Service methods for args testing

func (s *ServerTestService) MsgWithRequest(r *http.Request, req, resp *TestMsg) error {
	if r == nil {
		return errors.New("MsgWithRequest: r = nil")
	}
	resp.Name = req.Name
	return nil
}

func (s *ServerTestService) MsgWithContext(c Context, req, resp *TestMsg) error {
	if c == nil {
		return errors.New("MsgWithContext: c = nil")
	}
	resp.Name = req.Name
	return nil
}

func (s *ServerTestService) MsgWithReturn(c Context, req *TestMsg) (*TestMsg, error) {
	if c == nil {
		return nil, errors.New("MsgWithReturn: c = nil")
	}
	return &TestMsg{req.Name}, nil
}

func (s *ServerTestService) MsgWithoutRequest(c Context) (*TestMsg, error) {
	if c == nil {
		return nil, errors.New("MsgWithoutRequest: c = nil")
	}
	return &TestMsg{}, nil
}

func (s *ServerTestService) MsgWithoutResponse(c Context, req *TestMsg) error {
	if c == nil {
		return errors.New("MsgWithoutResponse: c = nil")
	}
	return nil
}

func (s *ServerTestService) MsgWithoutRequestNorResponse(c Context) error {
	if c == nil {
		return errors.New("MsgWithoutRequestNorResponse: c = nil")
	}
	return nil
}

func (s *ServerTestService) EchoRequest(r *http.Request, req *TestMsg) (*BytesMsg, error) {
	b, err := ioutil.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		return nil, err
	}
	return &BytesMsg{b}, nil
}

func createAPIServer() *Server {
	s := &ServerTestService{}
	rpc := &RPCService{
		name:     "ServerTestService",
		rcvr:     reflect.ValueOf(s),
		rcvrType: reflect.TypeOf(s),
		methods:  make(map[string]*ServiceMethod),
	}
	for i := 0; i < rpc.rcvrType.NumMethod(); i++ {
		m := rpc.rcvrType.Method(i)
		sm := &ServiceMethod{
			method:       &m,
			wantsContext: m.Type.In(1).Implements(typeOfContext),
		}
		if m.Type.NumIn() > 2 {
			sm.ReqType = m.Type.In(2).Elem()
		}
		if m.Type.NumOut() == 2 {
			sm.RespType = m.Type.Out(0).Elem()
		} else if m.Type.NumIn() > 3 {
			sm.RespType = m.Type.In(3).Elem()
		}
		rpc.methods[m.Name] = sm
	}

	smap := &serviceMap{services: make(map[string]*RPCService)}
	smap.services[rpc.name] = rpc
	return &Server{root: "/_ah/spi", services: smap}
}

func TestServerServeHTTP(t *testing.T) {
	server := createAPIServer()
	inst, err := aetest.NewInstance(nil)
	if err != nil {
		t.Fatalf("failed to create instance: %v", err)
	}
	defer inst.Close()

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
		{"POST", "CustomAPIError", `{}`, ``, http.StatusMethodNotAllowed},

		{"GET", "Void", `{}`, ``, http.StatusBadRequest},
		{"PUT", "Void", `{}`, ``, http.StatusBadRequest},
		{"HEAD", "Void", `{}`, ``, http.StatusBadRequest},
		{"DELETE", "Void", `{}`, ``, http.StatusBadRequest},
	}

	for i, tt := range tts {
		path := "/ServerTestService." + tt.srvMethod
		var body io.Reader
		if tt.httpVerb == "POST" || tt.httpVerb == "PUT" {
			body = strings.NewReader(tt.in)
		}
		var r *http.Request
		if r, err = inst.NewRequest(tt.httpVerb, path, body); err != nil {
			t.Fatalf("failed to create req: %v", r)
		}

		w := httptest.NewRecorder()

		// do the fake request
		server.ServeHTTP(w, r)

		// verify endpoints.context has been destroyed
		if c, exists := ctxs[r]; exists {
			t.Errorf("%d: ctxs[%#v] = %#v; want nil", i, r, c)
		}

		// make sure the response is correct
		out := strings.TrimSpace(w.Body.String())
		if tt.code == http.StatusOK && out != tt.out {
			t.Errorf("%d: %s %s = %q; want %q", i, tt.httpVerb, path, out, tt.out)
		}
		if w.Code != tt.code {
			t.Errorf("%d: %s %s w.Code = %d; want %d",
				i, tt.httpVerb, path, w.Code, tt.code)
		}
	}
}

func TestServerMethodCall(t *testing.T) {
	server := createAPIServer()
	inst, err := aetest.NewInstance(nil)
	if err != nil {
		t.Fatalf("failed to create instance: %v", err)
	}
	defer inst.Close()

	tts := []struct {
		name, body string
	}{
		{"MsgWithRequest", `{"name":"request"}`},
		{"MsgWithContext", `{"name":"context"}`},
		{"MsgWithReturn", `{"name":"return"}`},
	}

	for i, tt := range tts {
		path := "/ServerTestService." + tt.name
		body := strings.NewReader(tt.body)
		r, err := inst.NewRequest("POST", path, body)
		if err != nil {
			t.Fatalf("%d: failed to create req: %v", t, err)
		}

		w := httptest.NewRecorder()
		server.ServeHTTP(w, r)

		res := strings.TrimSpace(w.Body.String())
		if res != tt.body {
			t.Errorf("%d: %s res = %q; want %q", i, tt.name, res, tt.body)
		}
		if w.Code != http.StatusOK {
			t.Errorf("%d: %s code = %d; want %d", i, tt.name, w.Code, http.StatusOK)
		}
	}
}

func TestServerRegisterService(t *testing.T) {
	s, err := NewServer("").
		RegisterService(&ServerTestService{}, "ServerTestService", "v1", "", true)
	if err != nil {
		t.Fatalf("error registering service: %v", err)
	}

	tts := []struct {
		name         string
		wantsContext bool
		returnsResp  bool
	}{
		{"MsgWithRequest", false, false},
		{"MsgWithContext", true, false},
		{"MsgWithReturn", true, true},
	}
	for i, tt := range tts {
		m := s.MethodByName(tt.name)
		if m == nil {
			t.Errorf("%d: MethodByName(%q) = nil", i, tt.name)
			continue
		}
		if m.wantsContext != tt.wantsContext {
			t.Errorf("%d: wantsContext = %v; want %v", i, m.wantsContext, tt.wantsContext)
		}
	}
}

func TestServerMustRegisterService(t *testing.T) {
	s := NewServer("")

	var panicked interface{}
	func() {
		defer func() { panicked = recover() }()
		Must(s.RegisterService(&ServerTestService{}, "ServerTestService", "v1", "", true))
	}()
	if panicked != nil {
		t.Fatalf("unexpected panic: %v", panicked)
	}

	type badService struct{}
	func() {
		defer func() { panicked = recover() }()
		Must(s.RegisterService(&badService{}, "BadService", "v1", "", true))
	}()
	if panicked == nil {
		t.Fatalf("expected panic didn't occur")
	}
}

func TestServerRequestNotEmpty(t *testing.T) {
	server := createAPIServer()
	inst, err := aetest.NewInstance(nil)
	if err != nil {
		t.Fatalf("failed to create instance: %v", err)
	}
	defer inst.Close()

	path := "/ServerTestService.EchoRequest"
	body := `{"name": "francesc"}`
	r, err := inst.NewRequest("POST", path, strings.NewReader(body))
	if err != nil {
		t.Fatalf("failed to create req: %v", err)
	}

	w := httptest.NewRecorder()
	server.ServeHTTP(w, r)

	var res BytesMsg
	if err := json.NewDecoder(w.Body).Decode(&res); err != nil {
		t.Fatalf("decode response %q: %v", w.Body.String(), err)
	}

	if string(res.Bytes) != body {
		t.Fatalf("expected %q; got %q", body, res)
	}
}
