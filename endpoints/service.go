// Copyright 2009 The Go Authors. All rights reserved.
// Copyright 2012 The Gorilla Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package endpoints

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

var (
	// Precompute the reflect type for error.
	typeOfOsError = reflect.TypeOf((*error)(nil)).Elem()
	// Same as above, this time for http.Request.
	typeOfRequest = reflect.TypeOf((*http.Request)(nil)).Elem()
)

// ----------------------------------------------------------------------------
// service
// ----------------------------------------------------------------------------

// RpcService represents a service registered with a specific Server.
type RpcService struct {
	name     string                    // name of service
	rcvr     reflect.Value             // receiver of methods for the service
	rcvrType reflect.Type              // type of the receiver
	methods  map[string]*ServiceMethod // registered methods

	internal bool
	info     *ServiceInfo
}

// Name returns service method name
// TODO: remove or use info.Name here?
func (s *RpcService) Name() string {
	return s.name
}

// Info returns a ServiceInfo which is used to construct Endpoints API config
func (s *RpcService) Info() *ServiceInfo {
	return s.info
}

// Methods returns a slice of all service's registered methods
func (s *RpcService) Methods() []*ServiceMethod {
	items := make([]*ServiceMethod, 0, len(s.methods))
	for _, m := range s.methods {
		items = append(items, m)
	}
	return items
}

// MethodByName returns a ServiceMethod of a registered service's method or nil.
func (s *RpcService) MethodByName(name string) *ServiceMethod {
	return s.methods[name]
}

// ServiceInfo is used to construct Endpoints API config
type ServiceInfo struct {
	Name        string
	Version     string
	Default     bool
	Description string
}

// ServiceMethod is what represents a method of a registered service
type ServiceMethod struct {
	// Type of the request data structure
	ReqType reflect.Type
	// Type of the response data structure
	RespType reflect.Type
	// method's receiver
	method *reflect.Method
	// info used to construct Endpoints API config
	info *MethodInfo
}

// Info returns a MethodInfo struct of a registered service's method
func (m *ServiceMethod) Info() *MethodInfo {
	return m.info
}

// MethodInfo is what's used to construct Endpoints API config
type MethodInfo struct {
	// name can also contain resource, e.g. "greets.list"
	Name       string
	Path       string
	HttpMethod string
	Scopes     []string
	Audiences  []string
	ClientIds  []string
	Desc       string
}

// ----------------------------------------------------------------------------
// serviceMap
// ----------------------------------------------------------------------------

// serviceMap is a registry for services.
type serviceMap struct {
	mutex    sync.Mutex
	services map[string]*RpcService
}

// register adds a new service using reflection to extract its methods.
//
// internal == true indicase that this is an internal service,
// e.g. BackendService
func (m *serviceMap) register(srv interface{}, name, ver, desc string, isDefault, internal bool) (
	*RpcService, error) {

	// Setup service.
	s := &RpcService{
		rcvr:     reflect.ValueOf(srv),
		rcvrType: reflect.TypeOf(srv),
		methods:  make(map[string]*ServiceMethod),
		internal: internal,
	}
	s.name = reflect.Indirect(s.rcvr).Type().Name()
	if !isExported(s.name) {
		return nil, fmt.Errorf("endpoints: no service name for type %q",
			s.rcvrType.String())
	}

	if !internal {
		s.info = &ServiceInfo{
			Name:        name,
			Version:     ver,
			Default:     isDefault,
			Description: desc,
		}
		if s.info.Name == "" {
			s.info.Name = s.name
		}
		s.info.Name = strings.ToLower(s.info.Name)
		if s.info.Version == "" {
			s.info.Version = "v1"
		}
	}

	// Setup methods.
	for i := 0; i < s.rcvrType.NumMethod(); i++ {
		method := s.rcvrType.Method(i)
		srvMethod := newServiceMethod(&method, internal)
		if srvMethod != nil {
			s.methods[method.Name] = srvMethod
		}
	}
	if len(s.methods) == 0 {
		return nil, fmt.Errorf(
			"endpoints: %q has no exported methods of suitable type", s.name)
	}

	// Add to the map.
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.services == nil {
		m.services = make(map[string]*RpcService)
	} else if _, ok := m.services[s.name]; ok {
		return nil, fmt.Errorf("endpoints: service already defined: %q", s.name)
	}
	m.services[s.name] = s
	return s, nil
}

// newServiceMethod creates a new ServiceMethod from provided Go's Method.
//
// It doesn't create ServiceMethod.info if internal == true
func newServiceMethod(m *reflect.Method, internal bool) *ServiceMethod {
	mtype := m.Type
	// Method must be exported.
	// Method needs four ins: receiver, *http.Request, *args, *reply.
	if m.PkgPath != "" || mtype.NumIn() != 4 {
		return nil
	}
	// First argument must be a pointer and must be http.Request.
	reqType := mtype.In(1)
	if reqType.Kind() != reflect.Ptr || reqType.Elem() != typeOfRequest {
		return nil
	}
	// Second argument must be a pointer and must be exported.
	args := mtype.In(2)
	if args.Kind() != reflect.Ptr || !isExportedOrBuiltin(args) {
		return nil
	}
	// Third argument must be a pointer and must be exported.
	reply := mtype.In(3)
	if reply.Kind() != reflect.Ptr || !isExportedOrBuiltin(reply) {
		return nil
	}
	// Method needs one out: error.
	if mtype.NumOut() != 1 {
		return nil
	}
	if returnType := mtype.Out(0); returnType != typeOfOsError {
		return nil
	}

	method := &ServiceMethod{
		method:   m,
		ReqType:  args.Elem(),
		RespType: reply.Elem(),
	}
	if !internal {
		mname := strings.ToLower(m.Name)
		method.info = &MethodInfo{Name: mname}

		params := requiredParamNames(method.ReqType)
		numParam := len(params)
		if method.ReqType.Kind() == reflect.Struct {
			switch {
			default:
				method.info.HttpMethod = "POST"
			case numParam == method.ReqType.NumField():
				method.info.HttpMethod = "GET"
			}
		}
		if numParam == 0 {
			method.info.Path = mname
		} else {
			method.info.Path = mname + "/{" + strings.Join(params, "}/{") + "}"
		}
	}
	return method
}

// Used to infer method's info.Path.
// TODO: refactor this and move to apiconfig.go?
func requiredParamNames(t reflect.Type) []string {
	if t.Kind() == reflect.Struct {
		params := make([]string, 0, t.NumField())
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			// consider only exported fields
			if field.PkgPath == "" {
				parts := strings.Split(field.Tag.Get("endpoints"), ",")
				for _, p := range parts {
					if p == "required" {
						params = append(params, field.Name)
						break
					}
				}
			}
		}
		return params
	}
	return []string{}
}

// get returns a registered service given a method name.
//
// The method name uses a dotted notation as in "Service.Method".
func (m *serviceMap) get(method string) (*RpcService, *ServiceMethod, error) {
	parts := strings.Split(method, ".")
	if len(parts) != 2 {
		err := fmt.Errorf("endpoints: service/method request ill-formed: %q", method)
		return nil, nil, err
	}
	parts[1] = strings.Title(parts[1])

	m.mutex.Lock()
	service := m.services[parts[0]]
	m.mutex.Unlock()
	if service == nil {
		err := fmt.Errorf("endpoints: can't find service %q", parts[0])
		return nil, nil, err
	}
	ServiceMethod := service.methods[parts[1]]
	if ServiceMethod == nil {
		err := fmt.Errorf(
			"endpoints: can't find method %q of service %q", parts[1], parts[0])
		return nil, nil, err
	}
	return service, ServiceMethod, nil
}

// serviceByName returns a registered service or nil if there's no service
// registered by that name.
func (m *serviceMap) serviceByName(serviceName string) *RpcService {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.services[serviceName]
}

// isExported returns true of a string is an exported (upper case) name.
func isExported(name string) bool {
	rune, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(rune)
}

// isExportedOrBuiltin returns true if a type is exported or a builtin.
func isExportedOrBuiltin(t reflect.Type) bool {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	// PkgPath will be non-empty even for an exported type,
	// so we need to check the type name as well.
	return isExported(t.Name()) || t.PkgPath() == ""
}
