// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package blesser

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"v.io/x/ref/services/identity/internal/oauth"
	"v.io/x/ref/test/testutil"

	"v.io/v23/security"
)

func join(elements ...string) string {
	return strings.Join(elements, security.ChainSeparator)
}

func TestOAuthBlesser(t *testing.T) {
	var (
		provider, user = testutil.NewPrincipal(), testutil.NewPrincipal()
		ctx, call      = fakeContextAndCall(provider, user)
	)
	mockEmail := "testemail@example.com"
	blesser := NewOAuthBlesserServer(OAuthBlesserParams{
		OAuthProvider:    oauth.NewMockOAuth(mockEmail),
		BlessingDuration: time.Hour,
	})

	b, extension, err := blesser.BlessUsingAccessToken(ctx, call, "test-access-token")
	if err != nil {
		t.Errorf("BlessUsingAccessToken failed: %v", err)
	}

	wantExtension := join(mockEmail, oauth.MockClient)
	if extension != wantExtension {
		t.Errorf("got extension: %s, want: %s", extension, wantExtension)
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
	// "provider/testemail@example.com/test-client".
	user.AddToRoots(b)
	binfo := user.BlessingsInfo(b)
	if num := len(binfo); num != 1 {
		t.Errorf("Got blessings with %d names, want exactly one name", num)
	}
	if _, ok := binfo[join("provider", wantExtension)]; !ok {
		t.Errorf("BlessingsInfo %v does not have name %s", binfo, wantExtension)
	}
}
