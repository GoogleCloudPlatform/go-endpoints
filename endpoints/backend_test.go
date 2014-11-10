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
	r := newBackendHTTPRequest(inst, "GetApiConfigs", nil)
	req := &GetAPIConfigsRequest{AppRevision: "invalid"}

	if err := backend.GetApiConfigs(r, req, nil); err == nil {
		t.Errorf("GetApiConfigs(%#v) = nil; want error", req)
	}
}

func TestEmptyAPIConfigsList(t *testing.T) {
	inst, err := aetest.NewInstance(nil)
	if err != nil {
		t.Fatalf("failed to create instance: %v", err)
	}
	defer inst.Close()

	server := &Server{root: "/_ah/spi", services: new(serviceMap)}
	backend := newBackendService(server)
	r := newBackendHTTPRequest(inst, "GetApiConfigs", nil)
	req := &GetAPIConfigsRequest{}
	resp := &APIConfigsList{}

	if err := backend.GetApiConfigs(r, req, resp); err != nil {
		t.Errorf("GetApiConfigs() = %v", err)
	}
	if resp.Items == nil {
		t.Errorf("resp.Items = nil; want initialized")
	}
	if l := len(resp.Items); l != 0 {
		t.Errorf("len(resp.Item) = %d (%+v); want 0", l, resp.Items)
	}
}

func newBackendHTTPRequest(inst aetest.Instance, method string, body []byte) *http.Request {
	if body == nil {
		body = []byte{}
	}
	buf := bytes.NewBuffer(body)
	url := fmt.Sprintf("/_ah/spi/BackendService.%s", method)
	req, err := inst.NewRequest("POST", url, buf)
	if err != nil {
		log.Fatalf("failed to create req: %v", err)
	}
	return req
}
