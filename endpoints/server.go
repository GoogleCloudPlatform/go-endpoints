// Copyright 2009 The Go Authors. All rights reserved.
// Copyright 2012 The Gorilla Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package endpoints

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	// Mainly for debug logging
	"appengine"
	"io/ioutil"
)

// errorResponse is SPI-compatible error response
type errorResponse struct {
	// Currently always "APPLICATION_ERROR"
	State string `json:"state"`
	Name  string `json:"error_name"`
	Msg   string `json:"error_message,omitempty"`
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

// Creates and initializes a new errorResponse.
// If msg contains any of errorNames then errorResponse.Name will be set
// to that name and the rest of the msg becomes errorResponse.Msg.
// Otherwise, a default error name is used and msg argument
// is errorResponse.Msg.
func newErrorResponse(msg string) *errorResponse {
	if msg == "" {
		return &errorResponse{State: "APPLICATION_ERROR", Name: errorNames[0]}
	}
	err := &errorResponse{State: "APPLICATION_ERROR"}
	for _, name := range errorNames {
		if strings.HasPrefix(msg, name) {
			err.Name = name
			err.Msg = msg[len(name):]
		}
	}
	if err.Name == "" {
		err.Name = errorNames[0]
		err.Msg = msg
	}
	return err
}

// Server serves registered RPC services using registered codecs.
type Server struct {
	root     string
	services *serviceMap
}

// NewServer returns a new RPC server.
func NewServer(root string, registerBackend bool) *Server {
	if root == "" {
		root = "/_ah/spi/"
	} else if root[len(root)-1] != '/' {
		root += "/"
	}

	server := &Server{root: root, services: new(serviceMap)}
	// Don't register backend if we aren't using endpoints on GAE
	backend := newBackendService(server)
	if registerBackend {
		server.services.register(backend, "BackendService", "", "", true, true)
	} else {
		discovery := newDiscoveryService(server, backend)
		api, err := server.services.register(discovery, "discovery", "v1", "Discovery API", true, false)
		if err != nil {
			panic(err)
		}
		setupDiscoveryServiceMethods(api)
	}
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

// ServeHTTP is Server's implementation of http.ServiceByName interface.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Mainly for debug logging
	c := appengine.NewContext(r)

	// Split off the prefix from our path so we can find the methodName
	parts := strings.SplitN(r.URL.Path, s.root, 2)
	path := strings.Trim(parts[1], "/")

	// Always respond with JSON, even when an error occurs.
	// Note: API server doesn't expect an encoding in Content-Type header.
	w.Header().Set("Content-Type", "application/json")

	// Get service method specs
	serviceSpec, methodSpec, vars, err := s.services.getByPath(r, path)
	if err != nil {
		writeError(w, err)
		return
	}

	// Initialize RPC method request
	req := reflect.New(methodSpec.ReqType)

	for k, v := range *vars {
		titleKey := strings.Title(k)
		field := reflect.Indirect(req).FieldByName(titleKey)

		// Only accept strings as the target for path arguments
		if field.Kind() != reflect.String {
			continue
		}
		
		field.SetString(v)
	}

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
/*
	if err := json.Unmarshal(body, req.Interface()); err != nil {
		writeError(w, err)
		return
	}
*/

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
	json.NewEncoder(w).Encode(resp.Interface())
}

// writeError writes SPI-compatible error response.
func writeError(w http.ResponseWriter, err error) {
	errResp := newErrorResponse(err.Error())
	w.WriteHeader(400)
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
	DefaultServer = NewServer("", true)
}
