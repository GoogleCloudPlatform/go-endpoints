// Copyright 2009 The Go Authors. All rights reserved.
// Copyright 2012 The Gorilla Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package endpoints

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	// Mainly for debug logging
	"io/ioutil"
	"runtime/debug"
)

// This interface defines an error that is capable of setting an HTTP code.
type StatusError interface {
	error
	HTTPStatus() int
}

// errorResponse is SPI-compatible error response
type errorResponse struct {
	// Currently always "APPLICATION_ERROR"
	State string `json:"state"`
	Name  string `json:"error_name"`
	Msg   string `json:"error_message,omitempty"`
}

// This is the default error returned
const defaultError = http.StatusBadRequest

// errorNames is a slice of special error names (or better, their prefixes).
// First element is default error name.
// See newErrorResponse method for details.
var errorNames = map[int]string{
	http.StatusInternalServerError: "Internal Server Error",
	http.StatusBadRequest:          "Bad Request",
	http.StatusUnauthorized:        "Unauthorized",
	http.StatusForbidden:           "Forbidden",
	http.StatusNotFound:            "Not Found",
}

// Creates and initializes a new errorResponse.
// If msg contains any of errorNames then errorResponse.Name will be set
// to that name and the rest of the msg becomes errorResponse.Msg.
// Otherwise, a default error name is used and msg argument
// is errorResponse.Msg.
func newErrorResponse(msg string, status int) (*errorResponse, int) {
	err := &errorResponse{State: "APPLICATION_ERROR", Name: http.StatusText(defaultError), Msg: msg}
	if statusText := http.StatusText(status); len(statusText) > 0 {
		err.Name = statusText
		return err, status
	}
	for statusCode, prefix := range errorNames {
		if strings.HasPrefix(msg, prefix) {
			err.Name = prefix
			err.Msg = msg[len(prefix):]
			return err, statusCode
		}
	}
	return err, defaultError
}

// Server serves registered RPC services using registered codecs.
type Server struct {
	root     string
	services *serviceMap
}

// NewServer returns a new RPC server.
func NewServer(root string) *Server {
	if root == "" {
		root = "/_ah/spi/"
	} else if root[len(root)-1] != '/' {
		root += "/"
	}

	server := &Server{root: root, services: new(serviceMap)}
	backend := newBackendService(server)
	server.services.register(backend, "BackendService", "", "", true, true)
	return server
}

// RegisterService adds a new service to the server.
//
// The name parameter is optional: if empty it will be inferred from
// the receiver type name.
//
// Methods from the receiver will be extracted if these rules are satisfied:
//
//    - The receiver is exported (begins with an upper case letter) or local
//      (defined in the package registering the service).
//    - The method name is exported.
//    - The method has three arguments: *http.Request, *args, *reply.
//    - All three arguments are pointers.
//    - The second and third arguments are exported or local.
//    - The method has return type error.
//
// All other methods are ignored.
func (s *Server) RegisterService(srv interface{}, name, ver, desc string, isDefault bool) (*RpcService, error) {
	return s.services.register(srv, name, ver, desc, isDefault, false)
}

// RegisterServiceWithDefaults will register provided service and will try to
// infer Endpoints config params from its method names and types.
// See RegisterService for details.
func (s *Server) RegisterServiceWithDefaults(srv interface{}) (*RpcService, error) {
	return s.RegisterService(srv, "", "", "", true)
}

// ServiceByName returns a registered service or nil if there's no service
// registered by that name.
func (s *Server) ServiceByName(serviceName string) *RpcService {
	return s.services.serviceByName(serviceName)
}

// HandleHttp adds Server s to specified http.ServeMux.
// If no mux is provided http.DefaultServeMux will be used.
func (s *Server) HandleHttp(mux *http.ServeMux) {
	if mux == nil {
		mux = http.DefaultServeMux
	}
	mux.Handle(s.root, s)
}

// A type to contain arbitrary non-error values that are passed into panic()
type panicError struct {
	Message string
}

func (p *panicError) Error() string {
	return p.Message
}

func (p *panicError) HTTPStatus() int {
	return http.StatusInternalServerError
}

// ServeHTTP is Server's implementation of http.Handler interface.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c := NewContext(r)
	defer func() {
		destroyContext(c)
	}()

	// Always respond with JSON, even when an error occurs.
	// Note: API server doesn't expect an encoding in Content-Type header.
	w.Header().Set("Content-Type", "application/json")

	if r.Method != "POST" {
		err := fmt.Errorf("rpc: POST method required, got %q", r.Method)
		writeError(w, err)
		return
	}

	// methodName has "ServiceName.MethodName" format.
	var methodName string
	if idx := strings.LastIndex(r.URL.Path, "/"); idx < 0 {
		writeError(w, fmt.Errorf("rpc: no method in path %q", r.URL.Path))
		return
	} else {
		methodName = r.URL.Path[idx+1:]
	}

	// Get service method specs
	serviceSpec, methodSpec, err := s.services.get(methodName)
	if err != nil {
		writeError(w, err)
		return
	}

	// Initialize RPC method request
	req := reflect.New(methodSpec.ReqType)

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		writeError(w, err)
		return
	}
	c.Debugf("SPI request body: %s", body)

	// if err := json.NewDecoder(r.Body).Decode(req.Interface()); err != nil {
	// 	writeError(w, fmt.Errorf("Error while decoding JSON: %q", err))
	// 	return
	// }
	if err := json.Unmarshal(body, req.Interface()); err != nil {
		writeError(w, err)
		return
	}

	defer func() {
		if panicData := recover(); panicData != nil {
			c.Criticalf("Recovered from panic: %s\n%s", panicData, string(debug.Stack()))
			writeError(w, &panicError{
				Message: "An unknown internal error occured.",
			})
		}
	}()
	// Initialize RPC method response and call method's function
	resp := reflect.New(methodSpec.RespType)
	errValue := methodSpec.method.Func.Call([]reflect.Value{
		serviceSpec.rcvr,
		reflect.ValueOf(r),
		req,
		resp,
	})

	// Check if method returned an error
	if err := errValue[0].Interface(); err != nil {
		writeError(w, err.(error))
		return
	}

	// Encode non-error response
	if err := json.NewEncoder(w).Encode(resp.Interface()); err != nil {
		writeError(w, err)
	}
}

// writeError writes SPI-compatible error response.
func writeError(w http.ResponseWriter, err error) {
	var errResp *errorResponse
	var status int
	if statusError, ok := err.(StatusError); ok {
		desiredStatus := statusError.HTTPStatus()
		errResp, status = newErrorResponse(err.Error(), desiredStatus)
	} else {
		errResp, status = newErrorResponse(err.Error(), 0)
	}
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(errResp)
}

// DefaultServer is the default RPC server, so you don't have to explicitly
// create one.
var DefaultServer *Server

// RegisterService registers a service using DefaultServer.
// See Server.RegisterService for details.
func RegisterService(srv interface{}, name, ver, desc string, isDefault bool) (
	*RpcService, error) {

	return DefaultServer.RegisterService(srv, name, ver, desc, isDefault)
}

// RegisterServiceWithDefaults registers a service using DefaultServer.
// See Server.RegisterServiceWithDefaults for details.
func RegisterServiceWithDefaults(srv interface{}) (*RpcService, error) {
	return DefaultServer.RegisterServiceWithDefaults(srv)
}

// HandleHttp calls DefaultServer's HandleHttp method using default serve mux.
func HandleHttp() {
	DefaultServer.HandleHttp(nil)
}

// TODO: var DefaultServer = NewServer("") won't work so it's in the init()
// function for now.
func init() {
	DefaultServer = NewServer("")
}
