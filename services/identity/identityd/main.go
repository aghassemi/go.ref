// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The following enables go generate to generate the doc.go file.
//go:generate go run $V23_ROOT/release/go/src/v.io/x/lib/cmdline/testdata/gendoc.go . -help

package main

import (
	"database/sql"
	"fmt"
	"os"
	"time"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/x/lib/cmdline"
	"v.io/x/lib/vlog"
	"v.io/x/ref/lib/v23cmd"
	_ "v.io/x/ref/runtime/factories/static"
	"v.io/x/ref/services/identity/internal/auditor"
	"v.io/x/ref/services/identity/internal/blesser"
	"v.io/x/ref/services/identity/internal/caveats"
	"v.io/x/ref/services/identity/internal/oauth"
	"v.io/x/ref/services/identity/internal/revocation"
	"v.io/x/ref/services/identity/internal/server"
)

var (
	googleConfigWeb, googleConfigChrome, googleConfigAndroid         string
	externalHttpAddr, httpAddr, tlsConfig, assetsPrefix, mountPrefix string
)

func init() {
	// Configuration for various Google OAuth-based clients.
	cmdIdentityD.Flags.StringVar(&googleConfigWeb, "google-config-web", "", "Path to JSON-encoded OAuth client configuration for the web application that renders the audit log for blessings provided by this provider.")
	cmdIdentityD.Flags.StringVar(&googleConfigChrome, "google-config-chrome", "", "Path to the JSON-encoded OAuth client configuration for Chrome browser applications that obtain blessings from this server (via the OAuthBlesser.BlessUsingAccessToken RPC) from this server.")
	cmdIdentityD.Flags.StringVar(&googleConfigAndroid, "google-config-android", "", "Path to the JSON-encoded OAuth client configuration for Android applications that obtain blessings from this server (via the OAuthBlesser.BlessUsingAccessToken RPC) from this server.")

	// Flags controlling the HTTP server
	cmdIdentityD.Flags.StringVar(&externalHttpAddr, "external-http-addr", "", "External address on which the HTTP server listens on.  If none is provided the server will only listen on -http-addr.")
	cmdIdentityD.Flags.StringVar(&httpAddr, "http-addr", "localhost:8125", "Address on which the HTTP server listens on.")
	cmdIdentityD.Flags.StringVar(&tlsConfig, "tls-config", "", "Comma-separated list of TLS certificate and private key files, in that order.  This must be provided.")
	cmdIdentityD.Flags.StringVar(&assetsPrefix, "assets-prefix", "", "Host serving the web assets for the identity server.")
	cmdIdentityD.Flags.StringVar(&mountPrefix, "mount-prefix", "identity", "Mount name prefix to use.  May be rooted.")
}

func main() {
	cmdline.HideGlobalFlagsExcept()
	cmdline.Main(cmdIdentityD)
}

var cmdIdentityD = &cmdline.Command{
	Runner: v23cmd.RunnerFunc(runIdentityD),
	Name:   "identityd",
	Short:  "Runs HTTP server that creates security.Blessings objects",
	Long: `
Command identityd runs a daemon HTTP server that uses OAuth to create
security.Blessings objects.

Starts an HTTP server that brokers blessings after authenticating through OAuth.

To generate TLS certificates so the HTTP server can use SSL:
  go run $(go list -f {{.Dir}} "crypto/tls")/generate_cert.go --host <IP address>

To use Google as an OAuth provider the -google-config-* flags must be set to
point to the a JSON file obtained after registering the application with the
Google Developer Console at https://cloud.google.com/console

More details on Google OAuth at:
  https://developers.google.com/accounts/docs/OAuth2Login

More details on the design of identityd at:
  https://v.io/designdocs/identity-service.html
`,
}

func runIdentityD(ctx *context.T, env *cmdline.Env, args []string) error {
	var sqlDB *sql.DB
	var err error
	if sqlConf != "" {
		if sqlDB, err = dbFromConfigFile(sqlConf); err != nil {
			return env.UsageErrorf("Failed to create sqlDB: %v", err)
		}
	}

	googleoauth, err := oauth.NewGoogleOAuth(googleConfigWeb)
	if err != nil {
		return env.UsageErrorf("Failed to setup GoogleOAuth: %v", err)
	}

	auditor, reader, err := auditor.NewSQLBlessingAuditor(sqlDB)
	if err != nil {
		return fmt.Errorf("Failed to create sql auditor from config: %v", err)
	}

	revocationManager, err := revocation.NewRevocationManager(sqlDB)
	if err != nil {
		return fmt.Errorf("Failed to start RevocationManager: %v", err)
	}

	listenSpec := v23.GetListenSpec(ctx)
	s := server.NewIdentityServer(
		googleoauth,
		auditor,
		reader,
		revocationManager,
		googleOAuthBlesserParams(googleoauth, revocationManager),
		caveats.NewBrowserCaveatSelector(assetsPrefix),
		assetsPrefix,
		mountPrefix)
	s.Serve(ctx, &listenSpec, externalHttpAddr, httpAddr, tlsConfig)
	return nil
}

func googleOAuthBlesserParams(oauthProvider oauth.OAuthProvider, revocationManager revocation.RevocationManager) blesser.OAuthBlesserParams {
	params := blesser.OAuthBlesserParams{
		OAuthProvider:     oauthProvider,
		BlessingDuration:  365 * 24 * time.Hour,
		RevocationManager: revocationManager,
	}
	if clientID, err := getOAuthClientID(googleConfigChrome); err != nil {
		vlog.Info(err)
	} else {
		params.AccessTokenClients = append(params.AccessTokenClients, oauth.AccessTokenClient{Name: "chrome", ClientID: clientID})
	}
	if clientID, err := getOAuthClientID(googleConfigAndroid); err != nil {
		vlog.Info(err)
	} else {
		params.AccessTokenClients = append(params.AccessTokenClients, oauth.AccessTokenClient{Name: "android", ClientID: clientID})
	}
	return params
}

func getOAuthClientID(configFile string) (clientID string, err error) {
	f, err := os.Open(configFile)
	if err != nil {
		return "", fmt.Errorf("failed to open %q: %v", configFile, err)
	}
	defer f.Close()
	clientID, err = oauth.ClientIDFromJSON(f)
	if err != nil {
		return "", fmt.Errorf("failed to decode JSON in %q: %v", configFile, err)
	}
	return clientID, nil
}
