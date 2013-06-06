package endpoints

import (
	"fmt"
	"strings"
	"net/http"
	"encoding/json"
)

type DiscoveryRestDescription struct {
	Kind string `json:"kind"`
	Etag string `json:"etag,omitempty"`
	DiscoveryVersion string `json:"discoveryVersion"`
	Id string `json:"id"`
	Name string `json:"name"`
	Version string `json:"version"`
	DefaultVersion bool `json:"defaultVersion"`
	Description string `json:"description"`
	OwnerDomain string `json:"ownerDomain"`
	OwnerName string `json:"ownerName"`
	Icons map[string]interface{} `json:"icons"`

	Protocol string `json:"protocol"`
	BaseUrl string `json:"baseUrl"`
	BasePath string `json:"basePath"`
	RootUrl string `json:"rootUrl"`
	ServicePath string `json:"servicePath"`
	BatchPath string `json:"batchPath"`
	Parameters string `json:"parameters"`

	Schemas map[string]*ApiSchemaDescriptor `json:"schemas"`
	Resources  map[string]*DiscoveryResource `json:"resources"`
}

type DiscoveryResource struct {
	Methods map[string]*DiscoveryResourceMethod `json:"methods"`
}

type DiscoveryResourceMethod struct {
	Id string `json:"id"`
	Path string `json:"path"`
	HttpMethod string `json:"httpMethod"`
	Description string `json:"description"`
	Request    ApiReqRespDescriptor `json:"request"`
	Response   ApiReqRespDescriptor `json:"response"`
}

type DiscoveryRestIcons struct {
}

type DiscoveryRestResources struct {
}

func NewDiscoveryRestDescription() (d *DiscoveryRestDescription) {
	d = &DiscoveryRestDescription{}
	d.Kind = "discovery#restDescription"
	d.DiscoveryVersion = "v1"
	d.Protocol = "rest"
	d.Resources = make(map[string]*DiscoveryResource)
	return d
}

func NewDiscoveryResource() *DiscoveryResource {
	d := &DiscoveryResource{}
	d.Methods = make(map[string]*DiscoveryResourceMethod)
	return d
}

func DiscoveryFromApiConfig(api *ApiDescriptor) (d *DiscoveryRestDescription) {
	d = NewDiscoveryRestDescription()
	d.DefaultVersion = true

	d.Description = api.Desc
	d.Version = api.Version
	d.Name = api.Name

	d.Schemas = api.Descriptor.Schemas

	d.Id = fmt.Sprintf("%s:%s", d.Name, d.Version)

	for k, method := range api.Methods {
		resource := NewDiscoveryResource()
		d.Resources[method.Path] = resource
		t := &DiscoveryResourceMethod{}
		t.Id = k
		t.Path = method.Path
		t.HttpMethod = method.HttpMethod
		t.Description = method.Desc

		splitKey := strings.Split(k, ".")
		methodKey := splitKey[len(splitKey) - 1]
		resource.Methods[methodKey] = t
	}

	return
}

func GetApiDiscovery(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	s := newBackendService(DefaultServer)
	req := GetApiConfigsRequest{}
	apiResp := ApiConfigsList{}

	s.GetApiConfigs(r, &req, &apiResp)

	for _, service := range s.server.services.services {
		if service.internal {
			continue
		}
		d := &ApiDescriptor{}
		if err := service.ApiDescriptor(d, r.Host); err != nil {
			return
		}
		disc := DiscoveryFromApiConfig(d)
		bytes, _ := json.MarshalIndent(disc, " ", " ")
		w.Write(bytes)
		w.Write([]byte("\n\n\n\n\n\n"))
		bytes, _ = json.MarshalIndent(d, " ", " ")
		w.Write(bytes)

		w.Write(bytes)
	}

	return
}
