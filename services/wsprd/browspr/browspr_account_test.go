package browspr

import (
	"fmt"
	"reflect"
	"testing"

	"v.io/core/veyron2"
	"v.io/core/veyron2/context"
	"v.io/core/veyron2/ipc"
	"v.io/core/veyron2/security"

	_ "v.io/core/veyron/profiles"
)

const topLevelName = "mock-blesser"

type mockBlesserService struct {
	p     security.Principal
	count int
}

func newMockBlesserService(p security.Principal) *mockBlesserService {
	return &mockBlesserService{
		p:     p,
		count: 0,
	}
}

func (m *mockBlesserService) BlessUsingAccessToken(c *context.T, accessToken string, co ...ipc.CallOpt) (security.WireBlessings, string, error) {
	var empty security.WireBlessings
	m.count++
	name := fmt.Sprintf("%s%s%d", topLevelName, security.ChainSeparator, m.count)
	blessing, err := m.p.BlessSelf(name)
	if err != nil {
		return empty, "", err
	}
	return security.MarshalBlessings(blessing), name, nil
}

func setup(t *testing.T) (*Browspr, func()) {
	ctx, shutdown := veyron2.Init()

	spec := veyron2.GetListenSpec(ctx)
	spec.Proxy = "/mock/proxy"
	mockPostMessage := func(_ int32, _, _ string) {}
	browspr := NewBrowspr(ctx, mockPostMessage, &spec, "/mock:1234/identd", nil)
	principal := veyron2.GetPrincipal(browspr.ctx)
	browspr.accountManager.SetMockBlesser(newMockBlesserService(principal))

	return browspr, func() {
		browspr.Shutdown()
		shutdown()
	}
}

func TestHandleCreateAccount(t *testing.T) {
	browspr, teardown := setup(t)
	defer teardown()

	// Verify that HandleAuthGetAccountsRpc returns empty.
	nilValue := GetAccountsMessage{}
	a, err := browspr.HandleAuthGetAccountsRpc(nilValue)
	if err != nil {
		t.Fatal("browspr.HandleAuthGetAccountsRpc(%v) failed: %v", nilValue, err)
	}
	if len(a.([]string)) > 0 {
		t.Fatalf("Expected accounts to be empty array but got %v", a)
	}

	// Add one account.
	message1 := CreateAccountMessage{
		Token: "mock-access-token-1",
	}
	account1, err := browspr.HandleAuthCreateAccountRpc(message1)
	if err != nil {
		t.Fatalf("browspr.HandleAuthCreateAccountRpc(%v) failed: %v", message1, err)
	}

	// Verify that principalManager has the new account
	if b, err := browspr.principalManager.BlessingsForAccount(account1.(string)); err != nil || b == nil {
		t.Fatalf("Failed to get Blessings for account %v: got %v, %v", account1, b, err)
	}

	// Verify that HandleAuthGetAccountsRpc returns the new account.
	gotAccounts1, err := browspr.HandleAuthGetAccountsRpc(nilValue)
	if err != nil {
		t.Fatal("browspr.HandleAuthGetAccountsRpc(%v) failed: %v", nilValue, err)
	}
	if want := []string{account1.(string)}; !reflect.DeepEqual(want, gotAccounts1) {
		t.Fatalf("Expected account to be %v but got empty but got %v", want, gotAccounts1)
	}

	// Add another account
	message2 := CreateAccountMessage{
		Token: "mock-access-token-2",
	}
	account2, err := browspr.HandleAuthCreateAccountRpc(message2)
	if err != nil {
		t.Fatalf("browspr.HandleAuthCreateAccountsRpc(%v) failed: %v", message2, err)
	}

	// Verify that HandleAuthGetAccountsRpc returns the new account.
	gotAccounts2, err := browspr.HandleAuthGetAccountsRpc(nilValue)
	if err != nil {
		t.Fatal("browspr.HandleAuthGetAccountsRpc(%v) failed: %v", nilValue, err)
	}
	if want := []string{account1.(string), account2.(string)}; !reflect.DeepEqual(want, gotAccounts2) {
		t.Fatalf("Expected account to be %v but got empty but got %v", want, gotAccounts2)
	}

	// Verify that principalManager has both accounts
	if b, err := browspr.principalManager.BlessingsForAccount(account1.(string)); err != nil || b == nil {
		t.Fatalf("Failed to get Blessings for account %v: got %v, %v", account1, b, err)
	}
	if b, err := browspr.principalManager.BlessingsForAccount(account2.(string)); err != nil || b == nil {
		t.Fatalf("Failed to get Blessings for account %v: got %v, %v", account2, b, err)
	}
}

func TestHandleAssocAccount(t *testing.T) {
	browspr, teardown := setup(t)
	defer teardown()

	// First create an account.
	account := "mock-account"
	principal := veyron2.GetPrincipal(browspr.ctx)
	blessing, err := principal.BlessSelf(account)
	if err != nil {
		t.Fatalf("browspr.rt.Principal.BlessSelf(%v) failed: %v", account, err)
	}
	if err := browspr.principalManager.AddAccount(account, blessing); err != nil {
		t.Fatalf("browspr.principalManager.AddAccount(%v, %v) failed; %v", account, blessing, err)
	}

	origin := "https://my.webapp.com:443"

	// Verify that HandleAuthOriginHasAccountRpc returns false
	hasAccountMessage := OriginHasAccountMessage{
		Origin: origin,
	}
	hasAccount, err := browspr.HandleAuthOriginHasAccountRpc(hasAccountMessage)
	if err != nil {
		t.Fatal(err)
	}
	if hasAccount.(bool) {
		t.Fatal("Expected browspr.HandleAuthOriginHasAccountRpc(%v) to be false but was true", hasAccountMessage)
	}

	assocAccountMessage := AssociateAccountMessage{
		Account: account,
		Origin:  origin,
	}

	if _, err := browspr.HandleAuthAssociateAccountRpc(assocAccountMessage); err != nil {
		t.Fatalf("browspr.HandleAuthAssociateAccountRpc(%v) failed: %v", assocAccountMessage, err)
	}

	// Verify that HandleAuthOriginHasAccountRpc returns true
	hasAccount, err = browspr.HandleAuthOriginHasAccountRpc(hasAccountMessage)
	if err != nil {
		t.Fatal(err)
	}
	if !hasAccount.(bool) {
		t.Fatal("Expected browspr.HandleAuthOriginHasAccountRpc(%v) to be true but was false", hasAccountMessage)
	}

	// Verify that principalManager has the correct principal for the origin
	got, err := browspr.principalManager.Principal(origin)
	if err != nil {
		t.Fatalf("browspr.principalManager.Principal(%v) failed: %v", origin, err)
	}

	if got == nil {
		t.Fatalf("Expected browspr.principalManager.Principal(%v) to return a valid principal, but got %v", origin, got)
	}
}

func TestHandleAssocAccountWithMissingAccount(t *testing.T) {
	browspr, teardown := setup(t)
	defer teardown()

	account := "mock-account"
	origin := "https://my.webapp.com:443"
	message := AssociateAccountMessage{
		Account: account,
		Origin:  origin,
	}

	if _, err := browspr.HandleAuthAssociateAccountRpc(message); err == nil {
		t.Fatalf("browspr.HandleAuthAssociateAccountRpc(%v) should have failed but did not.")
	}

	// Verify that principalManager creates no principal for the origin
	got, err := browspr.principalManager.Principal(origin)
	if err == nil {
		t.Fatalf("Expected browspr.principalManager.Principal(%v) to fail, but got: %v", origin, got)
	}

	if got != nil {
		t.Fatalf("Expected browspr.principalManager.Principal(%v) not to return a principal, but got %v", origin, got)
	}

	// Verify that HandleAuthOriginHasAccountRpc returns false
	hasAccountMessage := OriginHasAccountMessage{
		Origin: origin,
	}
	hasAccount, err := browspr.HandleAuthOriginHasAccountRpc(hasAccountMessage)
	if err != nil {
		t.Fatal(err)
	}
	if hasAccount.(bool) {
		t.Fatal("Expected browspr.HandleAuthOriginHasAccountRpc(%v) to be false but was true", hasAccountMessage)
	}
}
