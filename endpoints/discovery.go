package endpoints

import (
	"fmt"
	"errors"
	"strings"
	"path"
	"net/http"
)

// DiscoveryDirectoryList describes a collection of APIs.
type DiscoveryDirectoryList struct {
	Kind string `json:"kind"`
	DiscoveryVersion string `json:"discoveryVersion"`
	Items []DiscoveryDirectoryItem `json:"items"`
}

func NewDiscoveryDirectoryItem() *DiscoveryDirectoryItem {
	return &DiscoveryDirectoryItem{}
}

// DiscoveryDirectoryItem describes where the discovery endpoint for an API can
// be found.
type DiscoveryDirectoryItem struct {
	Id string `json:"id"`
	Name string `json:"name"`
	Version string `json:"version"`
	Description string `json:"description"`
	DiscoveryRestUrl string `json:"discoveryRestUrl`
	DiscoveryLink string `json:"discoveryLink"`
	Icons map[string]string `json:"icons,omitempty"`
	Preferred bool `json:"preferred,omitempty"`
}

// DiscoveryRestDescription is the top-level struct for a
// discovery#restDescription document.
type DiscoveryRestDescription struct {
	Kind string `json:"kind"`
	Etag string `json:"etag,omitempty"`
	DiscoveryVersion string `json:"discoveryVersion"`
	Id string `json:"id"`
	Name string `json:"name"`
	Version string `json:"version"`
	DefaultVersion bool `json:"defaultVersion"`
	Description string `json:"description"`
	OwnerDomain string `json:"ownerDomain,omitempty"`
	OwnerName string `json:"ownerName,omitempty"`
	Icons map[string]string `json:"icons,omitempty"`

	Protocol string `json:"protocol"`
	BaseUrl string `json:"baseUrl,omitempty"`
	BasePath string `json:"basePath,omitempty"`
	RootUrl string `json:"rootUrl,omitempty"`
	ServicePath string `json:"servicePath"`
	BatchPath string `json:"batchPath,omitempty"`
	Parameters string `json:"parameters,omitempty"`

	Schemas map[string]*ApiSchemaDescriptor `json:"schemas"`
	Resources  map[string]*DiscoveryResource `json:"resources"`
}

// DiscoveryResource describes the resources on a DiscoveryRestDescription.
type DiscoveryResource struct {
	Methods map[string]*DiscoveryResourceMethod `json:"methods"`
}

// DiscoveryResourceMethod describes the rest method of a DiscoveryResource.
type DiscoveryResourceMethod struct {
	Id string `json:"id"`
	Path string `json:"path"`
	HttpMethod string `json:"httpMethod"`
	Description string `json:"description"`
	Request  *ApiSchemaRef `json:"request"`
	Response *ApiSchemaRef `json:"response"`
}

// Creates a new DiscoveryRestDescription with some default values
func NewDiscoveryRestDescription(rootUrl string, servicePath string) (d *DiscoveryRestDescription) {
	return d
}

func NewDiscoveryResource() *DiscoveryResource {
	d := &DiscoveryResource{}
	d.Methods = make(map[string]*DiscoveryResourceMethod)
	return d
}

// This provides the discovery service API without a GAE backend.
type DiscoveryService struct {
	server *Server // of which server
	backend *BackendService
	rootUrl string
	servicePath string
}

func newDiscoveryService(server *Server, backend *BackendService) *DiscoveryService {
	s := &DiscoveryService{}
	s.server = server
	s.backend = backend
	s.rootUrl = "rootUrl"
	s.servicePath = "servicePath"
	return s
}

func setupDiscoveryServiceMethods(api *RpcService) {
	info := api.MethodByName("List").Info()
	info.HttpMethod, info.Path, info.Desc =
		"GET", "apis", "List all apis."

	info = api.MethodByName("GetRest").Info()
	info.HttpMethod, info.Path, info.Desc =
		"GET", "api/{api}/{version}/rest", "List all apis."
}


func (s *DiscoveryService) directoryItemFromApiConfig(api *ApiDescriptor) (d *DiscoveryDirectoryItem) {
	d = NewDiscoveryDirectoryItem()

	d.Description = api.Desc
	d.Version = api.Version
	d.Name = api.Name
	d.Id = fmt.Sprintf("%s:%s", d.Name, d.Version)

	link := fmt.Sprintf("./apis/%s/%s/rest", d.Name, d.Version)
	d.DiscoveryLink = link
	d.DiscoveryRestUrl = fmt.Sprintf("%s/%s/v1/%s", s.rootUrl, s.servicePath, link)

	return d
}

func (s *DiscoveryService) descriptionFromApiConfig(api *ApiDescriptor, d *DiscoveryRestDescription) {
	d.Kind = "discovery#restDescription"
	d.DiscoveryVersion = "v1"
	d.Protocol = "rest"
	d.Resources = make(map[string]*DiscoveryResource)
	d.Icons = make(map[string]string)
	d.RootUrl = s.rootUrl
	d.ServicePath = s.servicePath

	d.BasePath = s.servicePath
	d.BaseUrl = s.rootUrl

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

		rosy := api.Descriptor.Methods[method.RosyMethod]
		t.Response = rosy.Response
		t.Request = rosy.Request

		splitKey := strings.Split(k, ".")
		methodKey := splitKey[len(splitKey) - 1]
		resource.Methods[methodKey] = t
	}

	return
}

type GetRestRequest struct {
	Api string `endpoints:"required"`
	Version string `endpoints:"required"`
}

// HandleApiRestDescription handles a request for a restDescription document
func (s *DiscoveryService) GetRest(
	r *http.Request, req *GetRestRequest, disc *DiscoveryRestDescription) error {
	service := s.server.services.servicePaths[path.Join(req.Api, req.Version)]
	if service == nil || service.internal {
		return errors.New("Invalid service")
	}
	d := &ApiDescriptor{}
	if err := service.ApiDescriptor(d, r.Host); err != nil {
		return err
	}
	s.descriptionFromApiConfig(d, disc)

	return nil
}

func (s *DiscoveryService) List(
	r *http.Request, _ *VoidMessage, resp *DiscoveryDirectoryList) error {

	apiReq := GetApiConfigsRequest{}
	apiResp := ApiConfigsList{}

	s.backend.GetApiConfigs(r, &apiReq, &apiResp)

	resp.Kind = "discovery#directoryList"
	resp.DiscoveryVersion = "v1"

	for _, service := range s.server.services.services {
		if service.internal {
			continue
		}
		d := &ApiDescriptor{}
		if err := service.ApiDescriptor(d, r.Host); err != nil {
			return err
		}
		item := s.directoryItemFromApiConfig(d)
		resp.Items = append(resp.Items, *item)
	}

	return nil
}

func HandleApi(w http.ResponseWriter, r *http.Request) {

}
