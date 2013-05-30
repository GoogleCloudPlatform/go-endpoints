package endpoints

import (
	"fmt"
	"testing"

	pb "appengine_internal/user"
	"code.google.com/p/goprotobuf/proto"

	tu "github.com/crhym3/aegot/testutils"
)

func TestCurrentBearerTokenUser(t *testing.T) {
	var (
		validScope    = "valid.scope"
		validClientId = "my-client-id"

		email      = "dude@gmail.com"
		userId     = "12345"
		authDomain = "gmail.com"
		isAdmin    = false

		empty = []string{}
	)

	getOAuthUser := func(in, out proto.Message, _ *tu.RpcCallOptions) error {
		scope := in.(*pb.GetOAuthUserRequest).GetScope()
		if scope != validScope {
			return fmt.Errorf("Invalid scope: %q", scope)
		}
		resp := out.(*pb.GetOAuthUserResponse)
		resp.ClientId = proto.String(validClientId)
		resp.Email = proto.String(email)
		resp.UserId = proto.String(userId)
		resp.AuthDomain = proto.String(authDomain)
		resp.IsAdmin = proto.Bool(isAdmin)
		return nil
	}
	unregister := tu.RegisterAPIOverride("user", "GetOAuthUser", getOAuthUser)
	defer unregister()

	req, deleteContext := tu.NewTestRequest("GET", "/", nil)
	defer deleteContext()

	tt := []*struct {
		scopes    []string
		clientIDs []string
		success   bool
	}{
		{empty, empty, false},
		{empty, []string{validClientId}, false},
		{[]string{validScope}, empty, false},
		{[]string{validScope}, []string{validClientId}, true},
		{[]string{"a", validScope, "b"}, []string{"c", validClientId, "d"}, true},
	}
	for _, elem := range tt {
		user, err := CurrentBearerTokenUser(req, elem.scopes, elem.clientIDs)
		switch {
		case elem.success && (err != nil || user == nil):
			t.Errorf("Did not expect the call to fail with "+
				"scopes=%v ids=%v. User: %+v, Error: %q",
				elem.scopes, elem.clientIDs, err, user)
		case !elem.success && err == nil:
			t.Errorf("Expected an error, got nil: scopes=%v ids=%v",
				elem.scopes, elem.clientIDs)
		}
	}

	scopes := []string{validScope}
	clientIDs := []string{validClientId}
	user, _ := CurrentBearerTokenUser(req, scopes, clientIDs)
	const failMsg = "Expected %q, got %q"
	if user.ID != userId {
		t.Errorf(failMsg, userId, user.ID)
	}
	if user.Email != email {
		t.Errorf(failMsg, email, user.Email)
	}
	if user.AuthDomain != authDomain {
		t.Errorf(failMsg, authDomain, user.AuthDomain)
	}
	if user.Admin != isAdmin {
		t.Errorf(failMsg, isAdmin, user.Admin)
	}
}
