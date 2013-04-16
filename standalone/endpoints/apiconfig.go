package endpoints

import "reflect"

var apis = make(map[string]*Api)

type Api struct {
	Name    string
	Version string
	methods map[string]*ApiMethod
}

// TODO: maybe merge ApiMethod and Api structs with discovery.go/RestMethod?
//       I guess I won't know 'till I finish the discovery doc and decide what
//       to do with gorilla/pat (third-party router)
type ApiMethod struct {
	HttpMethod string
	Path       string
	handler    reflect.Value
	reqType    reflect.Type
	respType   reflect.Type
	Name       string
}

// Method adds a new method to an API instance. Currently, handler *must* be
// a func with the following signature:
//
//   func myMethod(req *myReqStruct, resp *myRespStruct) (int, string)
//
// that is, two struct-pointer arguments and (code int, message string)
// as a return value.
//
// If returned code is not zero, it will be used as HTTP status code together
// with provided string message (as HTTP status text). If the message is not
// provided, a standard message for of the code will be used.
func (api *Api) Method(httpMethod, path string, handler interface{}, name string) *Api {
	hval := reflect.ValueOf(handler)
	htype := reflect.TypeOf(handler)
	reqType := htype.In(0)
	respType := htype.In(1)
	meth := &ApiMethod{httpMethod, path, hval, reqType.Elem(), respType.Elem(), name}
	api.methods[httpMethod+":"+path] = meth
	return api
}

// NewApi creates a new API description. Provided name and version together
// constructs the API ID.
func NewApi(name, version string) *Api {
	api := &Api{name, version, make(map[string]*ApiMethod)}
	apis[name+":"+version] = api
	return api
}
