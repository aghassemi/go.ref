// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package ref defines constants used through the Vanadium reference
// implementation, which is implemented in its subdirectories.
//
// For more details about the Vanadium project, please visit https://v.io.
package ref

import (
	"os"
	"strings"
)

const (
	// EnvCredentials is the name of the environment variable pointing to a
	// directory containing all the credentials of a principal (the blessing
	// store, the blessing roots, possibly the private key etc.).
	//
	// Typically only one of EnvCredentials or EnvAgentEndpoint will be set in a
	// process. If both are set, then EnvCredentials takes preference.
	//
	// See v.io/x/ref/lib/security.CreatePersistentPrincipal.
	EnvCredentials = "V23_CREDENTIALS"

	// EnvAgentEndpoint is the name of the environment variable pointing to an
	// agentd process containing all the credentials a principal (the blessing
	// store, the blessing roots, possibly the private key etc.).
	//
	// Typically only one of EnvCredentials or EnvAgentEndpoint will be set in a
	// process. If both are set, then EnvCredentials takes preference.
	EnvAgentEndpoint = "V23_AGENT_ENDPOINT"

	// EnvNamespacePrefix is the prefix of all environment variables that define a
	// namespace root.
	EnvNamespacePrefix = "V23_NAMESPACE"

	// EnvI18nCatalogueFiles is the name of the environment variable pointing to a
	// comma-separated list of i18n catalogue files to be loaded at startup.
	EnvI18nCatalogueFiles = "V23_I18N_CATALOGUE"

	// EnvOAuthIdentityProvider is the name of the environment variable pointing
	// to the url of the OAuth identity provider used by the principal
	// seekblessings command.
	EnvOAuthIdentityProvider = "V23_OAUTH_IDENTITY_PROVIDER"
)

// EnvNamespaceRoots returns the set of namespace roots to be used by the
// process, as specified by environment variables.
//
// It returns both a map of environment variable name to value and the list of
// values.
func EnvNamespaceRoots() (map[string]string, []string) {
	m := make(map[string]string)
	var l []string
	for _, ev := range os.Environ() {
		p := strings.SplitN(ev, "=", 2)
		if len(p) != 2 {
			continue
		}
		k, v := p[0], p[1]
		if strings.HasPrefix(k, EnvNamespacePrefix) && len(v) > 0 {
			l = append(l, v)
			m[k] = v
		}
	}
	return m, l
}

// EnvClearCredentials unsets all environment variables that are used by the
// Runtime to intialize the principal.
func EnvClearCredentials() error {
	for _, v := range []string{
		EnvCredentials,
		EnvAgentEndpoint,
	} {
		if err := os.Unsetenv(v); err != nil {
			return err
		}
	}
	return nil
}
