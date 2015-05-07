// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package oauth

const MockClient = "test-client"

// mockOAuth is a mock OAuthProvider for use in tests.
type mockOAuth struct {
	email string
}

func NewMockOAuth(mockEmail string) OAuthProvider {
	return &mockOAuth{email: mockEmail}
}

func (m *mockOAuth) AuthURL(redirectUrl string, state string, _ AuthURLApproval) string {
	return redirectUrl + "?state=" + state
}

func (m *mockOAuth) ExchangeAuthCodeForEmail(string, string) (string, error) {
	return m.email, nil
}

func (m *mockOAuth) GetEmailAndClientName(string, []AccessTokenClient) (string, string, error) {
	return m.email, MockClient, nil
}
