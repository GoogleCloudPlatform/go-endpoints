package endpoints

import (
	"encoding/json"
	"errors"
	"net/http"
)

type BackendService struct {
	server *Server // of which server
}

type ApiConfigs struct {
	Items []string `json:"items"`
}

// GetApiConfigs creates ApiDescriptor for every registered RpcService and
// responds with a config suitable for generating Discovery doc.
func (s *BackendService) GetApiConfigs(
	r *http.Request, _ *VoidMessage, resp *ApiConfigs) error {
	
	resp.Items = make([]string, 0)
	for _, service := range s.server.services.services {
		if service.internal {
			continue
		}
		d := &ApiDescriptor{}
		if err := service.ApiDescriptor(d, r.Host); err != nil {
			return err
		}
		bytes, err := json.Marshal(d)
		if err != nil {
			return err
		}
		resp.Items = append(resp.Items, string(bytes))
	}
	return nil
}

// This is a test method and will be removed sooner or later.
func (s *BackendService) GetFirstConfig(
	r *http.Request, _ *VoidMessage, resp *ApiDescriptor) error {
	
	for _, service := range s.server.services.services {
		if !service.internal {
			return service.ApiDescriptor(resp, r.Host)
		}
	}
	return errors.New("Not Found: No public API found")
}

func newBackendService(server *Server) *BackendService {
	return &BackendService{server: server}
}
