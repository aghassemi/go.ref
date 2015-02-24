package blesser

import (
	"reflect"
	"testing"
	"time"

	"v.io/core/veyron/services/identity/oauth"

	"v.io/v23/security"
)

func TestOAuthBlesser(t *testing.T) {
	var (
		provider, user = newPrincipal(), newPrincipal()
		context        = &serverCall{
			p:      provider,
			local:  blessSelf(provider, "provider"),
			remote: blessSelf(user, "self-signed-user"),
		}
	)
	blesser := NewOAuthBlesserServer(OAuthBlesserParams{
		OAuthProvider:    oauth.NewMockOAuth(),
		BlessingDuration: time.Hour,
	})

	result, extension, err := blesser.BlessUsingAccessToken(context, "test-access-token")
	if err != nil {
		t.Errorf("BlessUsingAccessToken failed: %v", err)
	}

	wantExtension := "users" + security.ChainSeparator + oauth.MockEmail + security.ChainSeparator + oauth.MockClient
	if extension != wantExtension {
		t.Errorf("got extension: %s, want: %s", extension, wantExtension)
	}

	b, err := security.NewBlessings(result)
	if err != nil {
		t.Fatalf("Unable to decode response into a security.Blessings object: %v", err)
	}
	if !reflect.DeepEqual(b.PublicKey(), user.PublicKey()) {
		t.Errorf("Received blessing for public key %v. Client:%v, Blesser:%v", b.PublicKey(), user.PublicKey(), provider.PublicKey())
	}

	// When the user does not recognize the provider, it should not see any strings for
	// the client's blessings.
	if got := user.BlessingsInfo(b); got != nil {
		t.Errorf("Got blessing with info %v, want nil", got)
	}
	// But once it recognizes the provider, it should see exactly the name
	// "provider/testemail@google.com/test-client".
	user.AddToRoots(b)
	binfo := user.BlessingsInfo(b)
	if num := len(binfo); num != 1 {
		t.Errorf("Got blessings with %d names, want exactly one name", num)
	}
	if _, ok := binfo["provider"+security.ChainSeparator+wantExtension]; !ok {
		t.Errorf("BlessingsInfo %v does not have name %s", binfo, wantExtension)
	}
}
