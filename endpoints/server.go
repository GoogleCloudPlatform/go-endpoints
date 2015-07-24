// Copyright 2009 The Go Authors. All rights reserved.
// Copyright 2012 The Gorilla Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package endpoints

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	// Mainly for debug logging
	"io/ioutil"
)

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
//    - The method has either 2 arguments and 2 return values:
//      *http.Request|Context, *arg => *reply, error
//      or 3 arguments and 1 return value:
//      *http.Request|Context, *arg, *reply => error
//    - The first argument is either *http.Request or Context.
//    - Second argument (*arg) and *reply are exported or local.
//    - First argument, *arg and *reply are all pointers.
//    - First (or second, if method has 2 arguments) return value is of type error.
//
// All other methods are ignored.
func (s *Server) RegisterService(srv interface{}, name, ver, desc string, isDefault bool) (*RPCService, error) {
	return s.services.register(srv, name, ver, desc, isDefault, false)
}

// RegisterServiceWithDefaults will register provided service and will try to
// infer Endpoints config params from its method names and types.
// See RegisterService for details.
func (s *Server) RegisterServiceWithDefaults(srv interface{}) (*RPCService, error) {
	return s.RegisterService(srv, "", "", "", true)
}

// Must is a helper that wraps a call to a function returning (*Template, error) and
// panics if the error is non-nil. It is intended for use in variable initializations
// such as:
// 	var s = endpoints.Must(endpoints.RegisterService(s, "Service", "v1", "some service", true))
//
func Must(s *RPCService, err error) *RPCService {
	if err != nil {
		panic(err)
	}
	return s
}

// ServiceByName returns a registered service or nil if there's no service
// registered by that name.
func (s *Server) ServiceByName(serviceName string) *RPCService {
	return s.services.serviceByName(serviceName)
}

// HandleHTTP adds Server s to specified http.ServeMux.
// If no mux is provided http.DefaultServeMux will be used.
func (s *Server) HandleHTTP(mux *http.ServeMux) {
	if mux == nil {
		mux = http.DefaultServeMux
	}
	mux.Handle(s.root, s)
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
	idx := strings.LastIndex(r.URL.Path, "/")
	if idx < 0 {
		writeError(w, fmt.Errorf("rpc: no method in path %q", r.URL.Path))
		return
	}
	methodName = r.URL.Path[idx+1:]

	// Get service method specs
	serviceSpec, methodSpec, err := s.services.get(methodName)
	if err != nil {
		writeError(w, err)
		return
	}

	// Initialize RPC method request
	reqValue := reflect.New(methodSpec.ReqType)

	body, err := ioutil.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		writeError(w, err)
		return
	}
	c.Debugf("SPI request body: %s", body)

	// if err := json.NewDecoder(r.Body).Decode(req.Interface()); err != nil {
	// 	writeError(w, fmt.Errorf("Error while decoding JSON: %q", err))
	// 	return
	// }
	if err := json.Unmarshal(body, reqValue.Interface()); err != nil {
		writeError(w, err)
		return
	}

	// Restore the body in the original request.
	r.Body = ioutil.NopCloser(bytes.NewReader(body))

	numIn, numOut := methodSpec.method.Type.NumIn(), methodSpec.method.Type.NumOut()
	// Construct arguments for the method call
	var httpReqOrCtx interface{} = r
	if methodSpec.wantsContext {
		httpReqOrCtx = c
	}
	args := []reflect.Value{serviceSpec.rcvr, reflect.ValueOf(httpReqOrCtx)}
	if numIn > 2 {
		args = append(args, reqValue)
	}

	var respValue reflect.Value
	if numIn > 3 {
		respValue = reflect.New(methodSpec.RespType)
		args = append(args, respValue)
	}

	// Invoke the service method
	var errValue reflect.Value
	res := methodSpec.method.Func.Call(args)
	if numOut == 2 {
		respValue = res[0]
		errValue = res[1]
	} else {
		errValue = res[0]
	}

	// Check if method returned an error
	if err := errValue.Interface(); err != nil {
		writeError(w, err.(error))
		return
	}

	// Encode non-error response
	if numIn == 4 || numOut == 2 {
		if err := json.NewEncoder(w).Encode(respValue.Interface()); err != nil {
			writeError(w, err)
		}
	}
}

// DefaultServer is the default RPC server, so you don't have to explicitly
// create one.
var DefaultServer *Server

// RegisterService registers a service using DefaultServer.
// See Server.RegisterService for details.
func RegisterService(srv interface{}, name, ver, desc string, isDefault bool) (
	*RPCService, error) {

	return DefaultServer.RegisterService(srv, name, ver, desc, isDefault)
}

// RegisterServiceWithDefaults registers a service using DefaultServer.
// See Server.RegisterServiceWithDefaults for details.
func RegisterServiceWithDefaults(srv interface{}) (*RPCService, error) {
	return DefaultServer.RegisterServiceWithDefaults(srv)
}

// HandleHTTP calls DefaultServer's HandleHTTP method using default serve mux.
func HandleHTTP() {
	DefaultServer.HandleHTTP(nil)
}

// TODO: var DefaultServer = NewServer("") won't work so it's in the init()
// function for now.
func init() {
	DefaultServer = NewServer("")
}
