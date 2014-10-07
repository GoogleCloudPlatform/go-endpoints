package endpoints

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"testing"

	"appengine/aetest"
)

func TestInvalidAppRevision(t *testing.T) {
	inst, err := aetest.NewInstance(nil)
	if err != nil {
		t.Fatalf("Failed to create instance: %v", err)
	}
	defer inst.Close()

	backend := &BackendService{}
	r := newBackendHttpRequest(inst, "GetApiConfigs", nil)
	req := &GetApiConfigsRequest{AppRevision: "invalid"}

	if err := backend.GetApiConfigs(r, req, nil); err == nil {
		t.Errorf("Expected GetApiConfigs to return an error")
	}
}

func TestEmptyApiConfigsList(t *testing.T) {
	inst, err := aetest.NewInstance(nil)
	if err != nil {
		t.Fatalf("Failed to create instance: %v", err)
	}
	defer inst.Close()

	server := &Server{root: "/_ah/spi", services: new(serviceMap)}
	backend := newBackendService(server)
	r := newBackendHttpRequest(inst, "GetApiConfigs", nil)
	req := &GetApiConfigsRequest{}
	resp := &ApiConfigsList{}

	if err := backend.GetApiConfigs(r, req, resp); err != nil {
		t.Error(err)
	}
	if resp.Items == nil {
		t.Errorf("Expected resp.Items to be initialized")
	}
	if len(resp.Items) != 0 {
		t.Errorf("Expected empty slice, got: %+v", resp.Items)
	}
}

func newBackendHttpRequest(inst aetest.Instance, method string, body []byte) *http.Request {
	if body == nil {
		body = []byte{}
	}
	buf := bytes.NewBuffer(body)
	url := fmt.Sprintf("/_ah/spi/BackendService.%s", method)
	req, err := inst.NewRequest("POST", url, buf)
	if err != nil {
		log.Fatalf("Failed to create req: %v", err)
	}
	return req
}
