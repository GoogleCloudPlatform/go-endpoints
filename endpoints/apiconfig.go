package endpoints

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"
)

// VoidMessage represents the fact that a service method does not expect
// anything in a request (or a response).
type VoidMessage struct{}

// ApiDescriptor is the top-level struct for a single Endpoints API config.
type ApiDescriptor struct {
	// Required
	Extends  string `json:"extends"`
	Root     string `json:"root"`
	Name     string `json:"name"`
	Version  string `json:"version"`
	Default  bool   `json:"defaultVersion"`
	Abstract bool   `json:"abstract"`
	Adapter  struct {
		Bns  string `json:"bns"`
		Type string `json:"type"`
	} `json:"adapter"`

	// Optional
	Cname string `json:"canonicalName,omitempty"`
	Desc  string `json:"description,omitempty"`
	Auth  *struct {
		AllowCookie bool `json:"allowCookieAuth"`
	} `json:"auth,omitempty"`

	// $METHOD_MAP
	Methods map[string]*ApiMethod `json:"methods"`

	// $SCHEMA_DESCRIPTOR
	Descriptor struct {
		Methods map[string]*ApiMethodDescriptor `json:"methods"`
		Schemas map[string]*ApiSchemaDescriptor `json:"schemas"`
	} `json:"descriptor"`
}

// ApiMethod is an item of $METHOD_MAP
type ApiMethod struct {
	Path       string               `json:"path"`
	HttpMethod string               `json:"httpMethod"`
	RosyMethod string               `json:"rosyMethod"`
	Request    ApiReqRespDescriptor `json:"request"`
	Response   ApiReqRespDescriptor `json:"response"`

	Scopes    []string `json:"scopes,omitempty"`
	Audiences []string `json:"audiences,omitempty"`
	ClientIds []string `json:"clientIds,omitempty"`
	Desc      string   `json:"description,omitempty"`
}

// ApiReqRespDescriptor indicates type of request data expected to be found
// in a request or a response.
type ApiReqRespDescriptor struct {
	Body     string                          `json:"body"`
	BodyName string                          `json:"bodyName,omitempty"`
	Params   map[string]*ApiRequestParamSpec `json:"parameters,omitempty"`
}

// ApiRequestParamSpec is a description of all the expected request parameters
// (if ApiReqRespDescriptor.Body is not "empty"?)
type ApiRequestParamSpec struct {
	Type     string                       `json:"type"`
	Required bool                         `json:"required,omitempty"`
	Default  interface{}                  `json:"default,omitempty"`
	Repeated bool                         `json:"repeated,omitempty"`
	Enum     map[string]*ApiEnumParamSpec `json:"enum,omitempty"`
}

type ApiEnumParamSpec struct {
	BackendVal string `json:"backendValue"`
	Desc       string `json:"description,omitempty"`
	// TODO: add 'number' field?
}

// ApiMethodDescriptor item of Descriptor.Methods map ($SCHEMA_DESCRIPTOR)
type ApiMethodDescriptor struct {
	Request  *ApiSchemaRef `json:"request,omitempty"`
	Response *ApiSchemaRef `json:"response,omitempty"`
	// Original method of an RpcService
	serviceMethod *ServiceMethod
}

type ApiSchemaRef struct {
	Ref string `json:"$ref"`
}

// ApiSchemaDescriptor item of Descriptor.Schemas map ($SCHEMA_DESCRIPTOR)
type ApiSchemaDescriptor struct {
	Id         string                        `json:"id"`
	Type       string                        `json:"type"`
	Properties map[string]*ApiSchemaProperty `json:"properties"`
	Desc       string                        `json:"description,omitempty"`
}

// ApiSchemaProperty is an item of ApiSchemaDescriptor.Properties map
type ApiSchemaProperty struct {
	Type   string             `json:"type,omitempty"`
	Format string             `json:"format,omitempty"`
	Items  *ApiSchemaProperty `json:"items,omitempty"`

	Required bool        `json:"required,omitempty"`
	Default  interface{} `json:"default,omitempty"`

	Ref  string `json:"$ref,omitempty"`
	Desc string `json:"description,omitempty"`
}

// ApiDescriptor populates provided ApiDescriptor with all info needed to
// generate a discovery doc from its receiver.
// 
// Args:
//   - dst, a non-nil pointer to ApiDescriptor struct
//   - host, a hostname used for discovery API config Root and BNS.
//   
// Returns error if malformed params were encountered
// (e.g. ServerMethod.Path, etc.)
func (s *RpcService) ApiDescriptor(dst *ApiDescriptor, host string) error {
	if dst == nil {
		return errors.New("Destination ApiDescriptor is nil")
	}
	if host == "" {
		return errors.New("Empty host parameter")
	}

	dst.Extends = "thirdParty.api"
	dst.Root = fmt.Sprintf("https://%s/_ah/api", host)
	dst.Name = s.Info().Name
	dst.Version = s.Info().Version
	dst.Default = s.Info().Default
	dst.Desc = s.Info().Description

	dst.Adapter.Bns = fmt.Sprintf("https://%s/_ah/spi", host)
	dst.Adapter.Type = "lily"

	schemasToCreate := make(map[string]reflect.Type, 0)
	methods := s.Methods()
	numMethods := len(methods)

	dst.Methods = make(map[string]*ApiMethod, numMethods)
	dst.Descriptor.Methods = make(map[string]*ApiMethodDescriptor, numMethods)

	for _, m := range methods {
		info := m.Info()

		// Methods of $SCHEMA_DESCRIPTOR
		mdescr := &ApiMethodDescriptor{serviceMethod: m}
		dst.Descriptor.Methods[s.Name()+"."+m.method.Name] = mdescr
		if !info.isBodiless() && !isEmptyStruct(m.ReqType) {
			refId := m.ReqType.Name()
			mdescr.Request = &ApiSchemaRef{Ref: refId}
			schemasToCreate[refId] = m.ReqType
		}
		if !isEmptyStruct(m.RespType) {
			refId := m.RespType.Name()
			mdescr.Response = &ApiSchemaRef{Ref: refId}
			schemasToCreate[refId] = m.RespType
		}

		// $METHOD_MAP
		apimeth, err := mdescr.toApiMethod(s.Name())
		if err != nil {
			return err
		}
		dst.Methods[dst.Name+"."+info.Name] = apimeth
	}

	// Schemas of $SCHEMA_DESCRIPTOR
	dst.Descriptor.Schemas = make(
		map[string]*ApiSchemaDescriptor, len(schemasToCreate))
	for _, t := range schemasToCreate {
		if err := addSchemaFromType(dst.Descriptor.Schemas, t); err != nil {
			return err
		}
	}
	return nil
}

// toApiMethod creates a new ApiMethod using its receiver info and provided
// rosy service name. 
// 
// Args:
//   - rosySrv, original name of a service, e.g. "MyService"
func (md *ApiMethodDescriptor) toApiMethod(rosySrv string) (*ApiMethod, error) {
	info := md.serviceMethod.Info()
	apim := &ApiMethod{
		Path:       info.Path,
		HttpMethod: info.HttpMethod,
		RosyMethod: rosySrv + "." + md.serviceMethod.method.Name,
		Scopes:     info.Scopes,
		Audiences:  info.Audiences,
		ClientIds:  info.ClientIds,
		Desc:       info.Desc,
	}

	var err error
	if md.serviceMethod.Info().isBodiless() {
		apim.Request.Params, err = typeToParamsSpec(md.serviceMethod.ReqType)
	} else {
		apim.Request.Params, err = typeToParamsSpecFromPath(
			md.serviceMethod.ReqType, apim.Path)
	}
	if err != nil {
		return nil, err
	}

	setApiReqRespBody(&apim.Request, "backendRequest", md.Request == nil)
	setApiReqRespBody(&apim.Response, "backendResponse", md.Response == nil)
	return apim, nil
}

// setApiReqRespBody populates ApiReqRespDescriptor with correct values based
// on provided arguments.
// 
// Args:
//   - d, a non-nil pointer of ApiReqRespDescriptor to populate
//   - template, either "backendRequest" or "backendResponse"
//   - empty, true if the origial method does not have a request/response body.
func setApiReqRespBody(d *ApiReqRespDescriptor, template string, empty bool) {
	if empty {
		d.Body = "empty"
	} else {
		d.Body = fmt.Sprintf("autoTemplate(%s)", template)
		d.BodyName = "resource"
	}
}

var (
	typeOfTime  = reflect.TypeOf(time.Time{})
	typeOfBytes = reflect.TypeOf([]byte(nil))
)

// addSchemaFromType creates a new ApiSchemaDescriptor from given Type t
// and adds it to the map with the key of type's name name.
// 
// If t Kind is a struct which has nested structs then... what happens then?
// 
// Returns an error if ApiSchemaDescriptor cannot be created from this Type.
func addSchemaFromType(dst map[string]*ApiSchemaDescriptor, t reflect.Type) error {
	if t.Name() == "" {
		return fmt.Errorf("Creating schema from unnamed type is currently not supported: %v", t)
	}
	if _, exists := dst[t.Name()]; exists {
		return nil
	}

	ensureSchemas := make(map[string]reflect.Type)
	sd := &ApiSchemaDescriptor{Id: t.Name()}

	switch t.Kind() {
	// case reflect.Array:
	// 	sd.Type = "array"
	// 	sd.Items... ?
	case reflect.Struct:
		sd.Type = "object"
		sd.Properties = make(map[string]*ApiSchemaProperty, t.NumField())
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			prop := &ApiSchemaProperty{}
			// TODO: add support for reflect.Struct and reflect.Map?
			switch field.Type.Kind() {
			default:
				prop.Type, prop.Format = typeToPropFormat(field.Type)
			case reflect.Ptr:
				prop.Ref = field.Type.Elem().Name()
			case reflect.Slice:
				if field.Type == typeOfBytes {
					prop.Type, prop.Format = "string", "byte"
					break
				}
				prop.Type = "array"
				prop.Items = &ApiSchemaProperty{}
				el := field.Type.Elem()
				if el.Kind() == reflect.Ptr {
					el = el.Elem()
				}
				k := el.Kind()
				// TODO: Add support for reflect.Map
				if k == reflect.Struct {
					prop.Items.Ref = el.Name()
					ensureSchemas[prop.Items.Ref] = el
				} else {
					prop.Items.Type, prop.Items.Format = typeToPropFormat(el)
				}
			}
			sd.Properties[field.Name] = prop
		}
	}

	dst[sd.Id] = sd

	for _, t := range ensureSchemas {
		if err := addSchemaFromType(dst, t); err != nil {
			return err
		}
	}

	return nil
}

// typeToPropFormat returns a pair of (item type, type format) strings
// for almost all kinds except for Struct, Map, ...
func typeToPropFormat(t reflect.Type) (string, string) {
	switch t.Kind() {
	default:
		// includes String and Array
		// TODO: Array is probably what should be enum type?
		return "string", ""
	case reflect.Bool:
		return "boolean", ""
	case reflect.Uint64:
		return "string", "uint64"
	case reflect.Int64:
		return "string", "int64"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32:
		return "integer", ""
	case reflect.Float32:
		return "number", "float"
	case reflect.Float64:
		return "number", "double"
	case reflect.Struct:
		// TODO: add support for other types?
		if t == typeOfTime {
			return "string", "date-time"
		}
	}

	return "", ""
}

// typeToParamsSpec creates a new ApiRequestParamSpec map from a Type.
// 
// Normally, t is a Struct type and it's what an original service method
// expects as input (request arg).
func typeToParamsSpec(t reflect.Type) (
	map[string]*ApiRequestParamSpec, error) {

	if t.Kind() != reflect.Struct {
		return nil, fmt.Errorf(
			"typeToParamsSpec: Only structs are supported, got: %v", t)
	}

	params := make(map[string]*ApiRequestParamSpec)
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		// consider only exported fields
		if field.PkgPath != "" {
			continue
		}
		param, err := fieldToParamSpec(&field)
		if err != nil {
			return nil, err
		}
		params[field.Name] = param
	}
	return params, nil
}

func typeToParamsSpecFromPath(t reflect.Type, path string) (
	map[string]*ApiRequestParamSpec, error) {

	if t.Kind() != reflect.Struct {
		return nil, fmt.Errorf(
			"typeToParamsSpec: Only structs are supported, got: %v", t)
	}

	params := make(map[string]*ApiRequestParamSpec)
	pathKeys, err := parsePath(path)
	if err != nil {
		return nil, err
	}
	for _, k := range pathKeys {
		k = strings.Title(k)
		field, found := t.FieldByName(k)
		if !found {
			return nil, fmt.Errorf(
				"typeToParamsSpec: Can't find field %q in %v (from path %q)", k, t, path)
		}
		param, err := fieldToParamSpec(&field)
		if err != nil {
			return nil, err
		}
		param.Required = true
		params[field.Name] = param
	}
	return params, nil
}

// parsePath parses a path template and returns found placeholders.
// It returns error if the template is malformed.
// 
// For instance, parsePath("one/{a}/two/{b}") will return []string{"a","b"}.
func parsePath(path string) ([]string, error) {
	params := make([]string, 0)
	for {
		i := strings.IndexRune(path, '{')
		if i < 0 {
			break
		}
		x := strings.IndexRune(path, '}')
		if x < i+1 {
			return nil, fmt.Errorf("parsePath: Invalid path template: %q", path)
		}
		params = append(params, path[i+1:x])
		path = path[x+1:]
	}
	return params, nil
}

// fieldToParamSpec creates a ApiRequestParamSpec from the given StructField.
// It returns error if the field's kind/type is not supported.
// 
// This method also checks for "endpoints" field tag options:
// TODO: describe "endpoints" field tag options.
func fieldToParamSpec(field *reflect.StructField) (*ApiRequestParamSpec, error) {
	param := &ApiRequestParamSpec{}
	switch field.Type.Kind() {
	default:
		param.Type = "string"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32:
		param.Type = "int32"
	case reflect.Int64:
		param.Type = "int64"
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32:
		param.Type = "uint32"
	case reflect.Uint64:
		param.Type = "uint64"
	case reflect.Float32:
		param.Type = "float"
	case reflect.Float64:
		param.Type = "double"
	case reflect.String:
		param.Type = "string"
	case reflect.Bool:
		param.Type = "boolean"
	case reflect.Slice:
		param.Repeated = true
		// TODO: set slice's Elem() type
	}
	if tag := field.Tag.Get("endpoints"); tag != "" {
		parts := strings.Split(tag, ",")
		for i, p := range parts {
			switch p {
			default:
				if i == 0 && p != "" {
					param.Default = p
				}
			case "required":
				param.Required = true
			}
		}
	}
	return param, nil
}

// isBodiless returns true of this is either GET or DELETE
func (info *MethodInfo) isBodiless() bool {
	// "OPTIONS" method is not supported anyway.
	return info.HttpMethod == "GET" || info.HttpMethod == "DELETE"
}

// isEmptyStruct returns true if given Type is either not a Struct or
// has 0 fields
func isEmptyStruct(t reflect.Type) bool {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	// TODO: check for unexported fields?
	return t.Kind() == reflect.Struct && t.NumField() == 0
}
