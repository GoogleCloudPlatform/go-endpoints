package endpoints

import (
	"encoding/json"
	"github.com/gorilla/pat"
	"net/http"
	"reflect"
	"strconv"
	"strings"
)

var router *pat.Router

type ErrorMessage struct {
	Code    int    `json:"code"`
	Message string `json:"message,omitempty"`
}

type ErrorResponse struct {
	Error ErrorMessage `json:"error"`
}

func (m ApiMethod) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	reqValPtr := reflect.New(m.reqType)
	if r.Method == "POST" || r.Method == "PUT" {
		json.NewDecoder(r.Body).Decode(reqValPtr.Interface())
	}

	reqVal := reqValPtr.Elem()
	for k, v := range r.URL.Query() {
		if k[0] == ':' {
			k = k[1:]
		}
		fieldVal := reqVal.FieldByName(strings.Title(k))
		if fieldVal.IsValid() && fieldVal.CanSet() {
			switch fieldVal.Kind() {
			case reflect.String:
				fieldVal.SetString(v[0])
			case reflect.Bool:
				if b, err := strconv.ParseBool(v[0]); err == nil {
					fieldVal.SetBool(b)
				}
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				if i, err := strconv.ParseInt(v[0], 0, 64); err == nil {
					fieldVal.SetInt(i)
				}
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				if n, err := strconv.ParseUint(v[0], 10, 64); err == nil {
					fieldVal.SetUint(n)
				}
			case reflect.Float32, reflect.Float64:
				if f, err := strconv.ParseFloat(v[0], 64); err == nil {
					fieldVal.SetFloat(f)
				}
			}
		}
	}

	respVal := reflect.New(m.respType)
	ret := m.handler.Call([]reflect.Value{reqValPtr, respVal})

	var resp interface{}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	code := int(ret[0].Int())
	if code > 299 {
		msg := ret[1].String()
		if msg == "" {
			msg = http.StatusText(code)
		}
		err := &ErrorResponse{}
		err.Error.Code, err.Error.Message = code, msg
		resp = err
		w.WriteHeader(code)
	} else {
		resp = respVal.Interface()
		if code > 0 {
			w.WriteHeader(code)
		}
	}

	json.NewEncoder(w).Encode(resp)
}

func Handle(prefix string) {
	if router == nil {
		router = pat.New()
	}

	if prefix[len(prefix)-1] != '/' {
		prefix += "/"
	}

	for _, api := range apis {
		pattern := prefix + api.Name + "/" + api.Version
		for _, meth := range api.methods {
			router.Add(meth.HttpMethod, pattern+meth.Path, meth)
		}
	}

	http.Handle(prefix, router)
}
