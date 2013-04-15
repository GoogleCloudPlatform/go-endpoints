package endpoints

import "reflect"

var apis = make(map[string]*Api)

type Api struct {
	Name    string
	Version string
	methods map[string]*ApiMethod
}

type ApiMethod struct {
	HttpMethod string
	Path       string
	handler    reflect.Value
	reqType    reflect.Type
	respType   reflect.Type
	Name       string
}

func (api *Api) Method(httpMethod, path string, handler interface{}, name string) *Api {
	hval := reflect.ValueOf(handler)
	htype := reflect.TypeOf(handler)
	reqType := htype.In(0)
	respType := htype.In(1)
	meth := &ApiMethod{httpMethod, path, hval, reqType.Elem(), respType.Elem(), name}
	api.methods[httpMethod+":"+path] = meth
	return api
}

func NewApi(name, version string) *Api {
	api := &Api{name, version, make(map[string]*ApiMethod)}
	apis[name+":"+version] = api
	return api
}
