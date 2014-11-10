package endpoints

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"appengine"
)

// Levels that can be specified for a LogMessage.
type logLevel string

const (
	levelDebug    logLevel = "debug"
	levelInfo     logLevel = "info"
	levelWarning  logLevel = "warning"
	levelError    logLevel = "error"
	levelCritical logLevel = "critical"
)

// GetAPIConfigsRequest is the request scheme for fetching API configs.
type GetAPIConfigsRequest struct {
	AppRevision string `json:"appRevision"`
}

// APIConfigsList is the response scheme for BackendService.getApiConfigs method.
type APIConfigsList struct {
	Items []string `json:"items"`
}

// LogMessagesRequest is the request body for log messages sent by Swarm FE.
type LogMessagesRequest struct {
	Messages []*LogMessage `json:"messages"`
}

// LogMessage is a single log message within a LogMessagesRequest.
type LogMessage struct {
	Level   logLevel `json:"level"`
	Message string   `json:"message" endpoints:"required"`
}

// BackendService is an API config enumeration service used by Google API Server.
//
// This is a simple API providing a list of APIs served by this App Engine
// instance. It is called by the Google API Server during app deployment
// to get an updated interface for each of the supported APIs.
type BackendService struct {
	server *Server // of which server
}

// GetApiConfigs creates APIDescriptor for every registered RPCService and
// responds with a config suitable for generating Discovery doc.
//
// Responds with a list of active APIs and their configuration files.
func (s *BackendService) GetApiConfigs(
	r *http.Request, req *GetAPIConfigsRequest, resp *APIConfigsList) error {
	c := appengine.NewContext(r)
	if req.AppRevision != "" {
		revision := strings.Split(appengine.VersionID(c), ".")[1]
		if req.AppRevision != revision {
			err := fmt.Errorf(
				"API backend app revision %s not the same as expected %s",
				revision, req.AppRevision)
			c.Errorf("%s", err)
			return err
		}
	}

	resp.Items = make([]string, 0)
	for _, service := range s.server.services.services {
		if service.internal {
			continue
		}
		d := &APIDescriptor{}
		if err := service.APIDescriptor(d, r.Host); err != nil {
			c.Errorf("%s", err)
			return err
		}
		bytes, err := json.Marshal(d)
		if err != nil {
			c.Errorf("%s", err)
			return err
		}
		resp.Items = append(resp.Items, string(bytes))
	}
	return nil
}

// LogMessages writes a log message from the Swarm FE to the log.
func (s *BackendService) LogMessages(
	r *http.Request, req *LogMessagesRequest, _ *VoidMessage) error {

	c := appengine.NewContext(r)
	for _, msg := range req.Messages {
		writeLogMessage(c, msg.Level, msg.Message)
	}
	return nil
}

// GetFirstConfig is a test method and will be removed sooner or later.
func (s *BackendService) GetFirstConfig(
	r *http.Request, _ *VoidMessage, resp *APIDescriptor) error {

	for _, service := range s.server.services.services {
		if !service.internal {
			return service.APIDescriptor(resp, r.Host)
		}
	}
	return errors.New("Not Found: No public API found")
}

func writeLogMessage(c appengine.Context, level logLevel, msg string) {
	const fmt = "%s"
	switch level {
	case levelDebug:
		c.Debugf(fmt, msg)
	case levelWarning:
		c.Warningf(fmt, msg)
	case levelError:
		c.Errorf(fmt, msg)
	case levelCritical:
		c.Criticalf(fmt, msg)
	default:
		c.Infof(fmt, msg)
	}
}

func newBackendService(server *Server) *BackendService {
	return &BackendService{server: server}
}
