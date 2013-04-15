package endpoints

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"
)

type JsonSchema struct {
	Id         string                 `json:"id,omitempty"`
	Type       string                 `json:"type,omitempty"`
	Format     string                 `json:"format,omitempty"`
	Ref        string                 `json:"$ref,omitempty"`
	Desc       string                 `json:"description,omitempty"`
	Properties map[string]*JsonSchema `json:"properties,omitempty"`
	ExtraProps *JsonSchema            `json:"additionalProperties,omitempty"`
	Items      *JsonSchema            `json:"items,omitempty"`
}

type RestMethod struct {
	Id          string
	HttpMethod  string
	Desc        string
	ParamsOrder []string `json:"parametersOrder,omitempty"`
}

type RestResource struct {
	Methods   map[string]*RestMethod   `json:"methods,omitempty"`
	Resources map[string]*RestResource `json:"resources,omitempty"`
}

type RestDescription struct {
	Id        string                   `json:"id"`
	Name      string                   `json:"name"`
	Version   string                   `json:"version"`
	Schemas   map[string]*JsonSchema   `json:"schemas"`
	Protocol  string                   `json:"protocol"`
	Methods   map[string]*RestMethod   `json:"methods,omitempty"`
	Resources map[string]*RestResource `json:"resources,omitempty"`
}

func simpleKindToStr(k reflect.Kind) (string, string) {
	switch k {
	case reflect.String:
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
	}
	return "", ""
}

func typeToSchema(schemas map[string]*JsonSchema, t reflect.Type) error {
	schemaId := t.Name()
	if _, exists := schemas[schemaId]; exists {
		return nil
	}

	var kind string
	switch t.Kind() {
	case reflect.Struct:
		kind = "object"
	case reflect.Array:
		kind = "array"
	default:
		return fmt.Errorf("Unsupported type: %s", t)
	}

	s := &JsonSchema{Id: schemaId, Type: kind}
	ensureRefs := make([]reflect.Type, 0)

	if t.Kind() == reflect.Struct {
		s.Properties = make(map[string]*JsonSchema)
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			prop := &JsonSchema{}
			switch field.Type.Kind() {
			default:
				prop.Type, prop.Format = simpleKindToStr(field.Type.Kind())
			case reflect.Struct:
				prop.Type = "object"
			case reflect.Map:
				prop.Type = "object"
				el := field.Type.Elem()
				if el.Kind() == reflect.Ptr {
					el = el.Elem()
				}
				prop.ExtraProps = &JsonSchema{Ref: el.Name()}
				ensureRefs = append(ensureRefs, el)
			case reflect.Array, reflect.Slice:
				prop.Type = "array"
				prop.Items = &JsonSchema{}
				el := field.Type.Elem()
				if el.Kind() == reflect.Ptr {
					el = el.Elem()
				}
				k := el.Kind()
				if k == reflect.Struct || k == reflect.Map {
					prop.Items.Ref = el.Name()
				} else {
					prop.Items.Type, prop.Items.Format = simpleKindToStr(k)
				}

			}
			s.Properties[strings.ToLower(field.Name)] = prop
		}
	}

	schemas[s.Id] = s

	for _, refType := range ensureRefs {
		if err := typeToSchema(schemas, refType); err != nil {
			return err
		}
	}

	return nil
}

type getRestRequest struct {
	Api     string
	Version string
}

func getRest(req *getRestRequest, doc *RestDescription) (int, string) {
	doc.Id = req.Api + ":" + req.Version
	doc.Name = req.Api
	doc.Version = req.Version
	doc.Schemas = make(map[string]*JsonSchema)
	doc.Protocol = "rest"

	api, exists := apis[doc.Id]
	if !exists {
		return http.StatusNotFound, ""
	}

	for _, meth := range api.methods {
		typeToSchema(doc.Schemas, meth.respType)
	}

	return 0, ""
}

func init() {
	discoApi := NewApi("discovery", "v1")
	discoApi.Method("GET", "/apis/{api}/{version}/rest", getRest, "apis.getRest")
	// TODO: list APIs (directory)
}
