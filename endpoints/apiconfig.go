package endpoints

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
)

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

// ApiRequestParamSpec is a description of all the expected request parameters.
type ApiRequestParamSpec struct {
	Type     string                       `json:"type"`
	Required bool                         `json:"required,omitempty"`
	Default  interface{}                  `json:"default,omitempty"`
	Repeated bool                         `json:"repeated,omitempty"`
	Enum     map[string]*ApiEnumParamSpec `json:"enum,omitempty"`
	// only for int32/int64/uint32/uint64
	Min interface{} `json:"minValue,omitempty"`
	Max interface{} `json:"maxValue,omitempty"`
}

type ApiEnumParamSpec struct {
	BackendVal string `json:"backendValue"`
	Desc       string `json:"description,omitempty"`
	// TODO: add 'number' field?
}

// ApiMethodDescriptor item of Descriptor.Methods map ($SCHEMA_DESCRIPTOR).
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

// addSchemaFromType creates a new ApiSchemaDescriptor from given Type t
// and adds it to the map with the key of type's name name.
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
		fieldsMap := fieldNames(t, false)
		sd.Properties = make(map[string]*ApiSchemaProperty, len(fieldsMap))
		sd.Type = "object"
		for name, field := range fieldsMap {
			fkind := field.Type.Kind()
			prop := &ApiSchemaProperty{}

			// TODO(alex): add support for reflect.Map?
			switch {
			default:
				prop.Type, prop.Format = typeToPropFormat(field.Type)

			case implements(field.Type, typeOfJsonMarshaler):
				prop.Type = "string"

			case fkind == reflect.Ptr, fkind == reflect.Struct:
				typ := indirectType(field.Type)
				if stype, format := typeToPropFormat(typ); stype != "" {
					// pointer to a basic type.
					prop.Type, prop.Format = stype, format
				} else {
					switch {
					case typ == typeOfTime:
						prop.Type, prop.Format = "string", "date-time"

					case typ.Kind() == reflect.Struct:
						prop.Ref = typ.Name()
						ensureSchemas[prop.Ref] = typ
					default:
						return fmt.Errorf(
							"Unsupported type %#v of property %s.%s",
							field.Type, sd.Id, name)
					}
				}

			case fkind == reflect.Slice:
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
				// TODO(alex): Add support for reflect.Map?
				if k == reflect.Struct {
					prop.Items.Ref = el.Name()
					ensureSchemas[prop.Items.Ref] = el
				} else {
					prop.Items.Type, prop.Items.Format = typeToPropFormat(el)
				}
			}

			tag, err := parseTag(field.Tag)
			if err != nil {
				return err
			}
			prop.Required = tag.required
			prop.Desc = tag.desc
			prop.Default, err = parseValue(tag.defaultVal, field.Type.Kind())
			if err != nil {
				return err
			}

			sd.Properties[name] = prop
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

// isBodiless returns true of this is either GET or DELETE
func (info *MethodInfo) isBodiless() bool {
	// "OPTIONS" method is not supported anyway.
	return info.HttpMethod == "GET" || info.HttpMethod == "DELETE"
}

// ---------------------------------------------------------------------------
// Types

// VoidMessage represents the fact that a service method does not expect
// anything in a request (or a response).
type VoidMessage struct{}

type jsonMarshaler interface {
	json.Marshaler
	json.Unmarshaler
}

var (
	typeOfTime          = reflect.TypeOf(time.Time{})
	typeOfBytes         = reflect.TypeOf([]byte(nil))
	typeOfJsonMarshaler = reflect.TypeOf((*jsonMarshaler)(nil)).Elem()
)

// indirectType returns a type the t is pointing to or a type of the element
// of t if t is either Array, Chan, Map or Slice.
func indirectType(t reflect.Type) reflect.Type {
	switch t.Kind() {
	case reflect.Array, reflect.Chan, reflect.Map, reflect.Ptr, reflect.Slice:
		return t.Elem()
	}
	return t
}

// indirectKind returns kind of a type the t is pointing to.
func indirectKind(t reflect.Type) reflect.Kind {
	return indirectType(t).Kind()
}

// implements returns true if Type t implements interface of Type impl.
func implements(t reflect.Type, impl reflect.Type) bool {
	return t.Implements(impl) || indirectType(t).Implements(impl)
}

// isEmptyStruct returns true if given Type is either not a Struct or
// has 0 fields
func isEmptyStruct(t reflect.Type) bool {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	// TODO(alex): check for unexported fields?
	return t.Kind() == reflect.Struct && t.NumField() == 0
}

// typeToPropFormat returns a pair of (item type, type format) strings
// for "simple" kinds.
func typeToPropFormat(t reflect.Type) (string, string) {
	switch t.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32:
		return "integer", "int32"
	case reflect.Int64:
		return "string", "int64"
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32:
		return "integer", "uint32"
	case reflect.Uint64:
		return "string", "uint64"
	case reflect.Float32:
		return "number", "float"
	case reflect.Float64:
		return "number", "double"
	case reflect.Bool:
		return "boolean", ""
	case reflect.String:
		return "string", ""
	}

	return "", ""
}

// typeToParamsSpec creates a new ApiRequestParamSpec map from a Type for all
// fields in t.
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

	for name, field := range fieldNames(t, true) {
		param, err := fieldToParamSpec(field)
		if err != nil {
			return nil, err
		}
		params[name] = param
	}

	return params, nil
}

// typeToParamsSpecFromPath is almost the same as typeToParamsSpec but considers
// only those params present in template path.
//
// path template is is something like "some/{a}/path/{b}".
func typeToParamsSpecFromPath(t reflect.Type, path string) (
	map[string]*ApiRequestParamSpec, error) {

	if t.Kind() != reflect.Struct {
		return nil, fmt.Errorf(
			"typeToParamsSpecFromPath: Only structs are supported, got: %v", t)
	}

	pathKeys, err := parsePath(path)
	if err != nil {
		return nil, err
	}

	fieldsMap := fieldNames(t, true)
	params := make(map[string]*ApiRequestParamSpec)

	for _, k := range pathKeys {
		field, found := fieldsMap[k]
		if !found {
			return nil, fmt.Errorf(
				"typeToParamsSpecFromPath: Can't find field %q in %v (from path %q)",
				k, t, path)
		}
		param, err := fieldToParamSpec(field)
		if err != nil {
			return nil, err
		}
		param.Required = true
		params[k] = param
	}

	return params, nil
}

// fieldToParamSpec creates a ApiRequestParamSpec from the given StructField.
// It returns error if the field's kind/type is not supported.
//
// See parseTag() method for supported tag options.
func fieldToParamSpec(field *reflect.StructField) (p *ApiRequestParamSpec, err error) {
	p = &ApiRequestParamSpec{}
	kind := field.Type.Kind()
	if kind == reflect.Ptr {
		kind = indirectType(field.Type).Kind()

	}
	switch {
	case reflect.Int <= kind && kind <= reflect.Int32:
		p.Type = "int32"
	case kind == reflect.Int64:
		p.Type = "int64"
	case reflect.Uint <= kind && kind <= reflect.Uint32:
		p.Type = "uint32"
	case kind == reflect.Uint64:
		p.Type = "uint64"
	case kind == reflect.Float32:
		p.Type = "float"
	case kind == reflect.Float64:
		p.Type = "double"
	case kind == reflect.Bool:
		p.Type = "boolean"
	case field.Type == typeOfBytes:
		p.Type = "bytes"
	case kind == reflect.String, implements(field.Type, typeOfJsonMarshaler):
		p.Type = "string"
	default:
		return nil, fmt.Errorf("Unsupported field: %#v", field)
	}

	var tag *endpointsTag
	if tag, err = parseTag(field.Tag); err != nil {
		return nil, fmt.Errorf("Tag error on %#v: %s", field, err)
	}

	p.Required = tag.required
	if p.Default, err = parseValue(tag.defaultVal, kind); err != nil {
		return
	}
	if reflect.Int <= kind && kind <= reflect.Uint64 {
		p.Min, err = parseValue(tag.minVal, kind)
		if err != nil {
			return
		}
		p.Max, err = parseValue(tag.maxVal, kind)
	}

	return
}

// fieldNames loops over each field of t and creates a map of
// fieldName (string) => *StructField where fieldName is extracted from json
// field tag. Defaults to StructField.Name.
//
// It expands (flattens) nexted structs if flatten == true, and always skips
// unexported fields or thosed tagged with json:"-"
//
// This method accepts only reflect.Struct type. Passing other types will
// most likely make it panic.
func fieldNames(t reflect.Type, flatten bool) map[string]*reflect.StructField {
	numField := t.NumField()
	m := make(map[string]*reflect.StructField, numField)

	for i := 0; i < numField; i++ {
		f := t.Field(i)
		// consider only exported fields
		if f.PkgPath != "" {
			continue
		}

		name := strings.Split(f.Tag.Get("json"), ",")[0]
		if name == "-" {
			continue
		} else if name == "" {
			name = f.Name
		}

		if flatten && indirectKind(f.Type) == reflect.Struct &&
			!implements(f.Type, typeOfJsonMarshaler) {

			for nname, nfield := range fieldNames(indirectType(f.Type), true) {
				m[name+"."+nname] = nfield
			}
			continue
		}

		m[name] = &f
	}

	return m
}

// ----------------------------------------------------------------------------
// Parse

type endpointsTag struct {
	required                   bool
	defaultVal, minVal, maxVal string
	desc                       string
}

const endpointsTagName = "endpoints"

// parseTag parses "endpoints" field tag into endpointsTag struct.
//
//   type MyMessage struct {
//       SomeField int `endpoints:"req,min=0,max=100,desc="Int field"`
//       WithDefault string `endpoints:"d=Hello gopher"`
//   }
//
//   - req, required (boolean)
//   - d=val, default value
//   - min=val, min value
//   - max=val, max value
//   - desc=val, description
//
// It is an error to specify both default and required.
func parseTag(t reflect.StructTag) (*endpointsTag, error) {
	eTag := &endpointsTag{}
	if tag := t.Get("endpoints"); tag != "" {
		parts := strings.Split(tag, ",")
		for _, k := range parts {
			switch k {
			case "req":
				eTag.required = true
			default:
				// key=value format
				kv := strings.SplitN(k, "=", 2)
				if len(kv) < 2 {
					continue
				}
				switch kv[0] {
				case "d":
					eTag.defaultVal = kv[1]
				case "min":
					eTag.minVal = kv[1]
				case "max":
					eTag.maxVal = kv[1]
				case "desc":
					eTag.desc = kv[1]
				}
			}
		}
		if eTag.required && eTag.defaultVal != "" {
			return nil, fmt.Errorf(
				"Can't have both required and default (%#v)",
				eTag.defaultVal)
		}
	}
	return eTag, nil
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

// parseValue parses string s into its "real" value of kind k.
// Only these kinds are supported: (u)int8/16/32/64, float32/64, bool, string.
func parseValue(s string, k reflect.Kind) (interface{}, error) {
	if s == "" {
		return nil, nil
	}

	switch {
	case reflect.Int <= k && k <= reflect.Int32:
		return strconv.Atoi(s)
	case k == reflect.Int64:
		return strconv.ParseInt(s, 0, 64)
	case reflect.Uint <= k && k <= reflect.Uint32:
		if v, err := strconv.ParseUint(s, 0, 32); err == nil {
			return uint32(v), nil
		} else {
			return nil, err
		}
	case k == reflect.Uint64:
		return strconv.ParseUint(s, 0, 64)
	case k == reflect.Float32:
		if v, err := strconv.ParseFloat(s, 32); err == nil {
			return float32(v), nil
		} else {
			return nil, err
		}
	case k == reflect.Float64:
		return strconv.ParseFloat(s, 64)
	case k == reflect.Bool:
		return strconv.ParseBool(s)
	case k == reflect.String:
		return s, nil
	}

	return nil, fmt.Errorf("parseValue: Invalid kind %#v value=%q", k, s)
}
