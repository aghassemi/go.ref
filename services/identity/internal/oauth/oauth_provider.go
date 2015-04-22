// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package oauth

// AccessTokenClient represents a client of an OAuthProvider.
type AccessTokenClient struct {
	// Descriptive name of the client.
	Name string
	// OAuth Client ID.
	ClientID string
}

// Option to OAuthProvider.AuthURL controlling whether previously provided user consent can be re-used.
type AuthURLApproval bool

const (
	ExplicitApproval AuthURLApproval = false // Require explicit user consent.
	ReuseApproval    AuthURLApproval = true  // Reuse a previous user consent if possible.
)

// OAuthProvider authenticates users to the identity server via the OAuth2 Web Server flow.
type OAuthProvider interface {
	// AuthURL is the URL the user must visit in order to authenticate with the OAuthProvider.
	// After authentication, the user will be re-directed to redirectURL with the provided state.
	AuthURL(redirectUrl string, state string, approval AuthURLApproval) (url string)
	// ExchangeAuthCodeForEmail exchanges the provided authCode for the email of the
	// authenticated user on behalf of the token has been issued.
	ExchangeAuthCodeForEmail(authCode string, url string) (email string, err error)
	// GetEmailAndClientName verifies that the provided 'accessToken' is issued to one
	// of the provided accessTokenClients, and if so returns the email of the
	// authenticated user on behalf of whom the token has been issued, and also the
	// client name associated with the token.
	GetEmailAndClientName(accessToken string, accessTokenClients []AccessTokenClient) (email string, clientName string, err error)
}
