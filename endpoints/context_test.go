package endpoints

import (
	"testing"

	"google.golang.org/appengine/internal"

	basepb "appengine_internal/base"
)

func TestCachingContextNamespace(t *testing.T) {
	const namespace = "separated"

	r, _, cleanup := newTestRequest(t, "GET", "/", nil)
	defer cleanup()
	c := cachingContextFactory(r)
	nc, err := c.Namespace(namespace)
	if err != nil {
		t.Fatalf("Namespace() returned error: %v", err)
	}
	ns := &basepb.StringProto{}
	if err := internal.Call(nc, "__go__", "GetNamespace", &basepb.VoidProto{}, ns); err != nil {
		t.Fatalf("Error calling __go__.GetNamespace: %v", err)
	}
	if namespace != ns.GetValue() {
		t.Errorf("expected ns %q, got %q", namespace, ns.GetValue())
	}
}
