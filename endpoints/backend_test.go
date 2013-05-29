package endpoints

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"testing"

	"github.com/crhym3/aegot/testutils"
)

func TestInvalidAppRevision(t *testing.T) {
	backend := &BackendService{}
	r := newBackendHttpRequest("GetApiConfigs", nil)
	req := &GetApiConfigsRequest{AppRevision: "invalid"}

	if err := backend.GetApiConfigs(r, req, nil); err == nil {
		t.Errorf("Expected GetApiConfigs to return an error")
	}
}

func TestEmptyApiConfigsList(t *testing.T) {
	server := &Server{root: "/_ah/spi", services: new(serviceMap)}
	backend := newBackendService(server)
	r := newBackendHttpRequest("GetApiConfigs", nil)
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

func newBackendHttpRequest(method string, body []byte) *http.Request {
	if body == nil {
		body = []byte{}
	}
	buf := bytes.NewBuffer(body)
	url := fmt.Sprintf("/_ah/spi/BackendService.%s", method)
	req, err := http.NewRequest("POST", url, buf)
	if err != nil {
		log.Fatal(err)
	}
	testutils.CreateTestContext(req)
	return req
}
