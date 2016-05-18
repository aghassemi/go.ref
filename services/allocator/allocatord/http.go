// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"v.io/v23/context"
	"v.io/v23/security"
)

const (
	routeRoot    = "/"
	routeHome    = "/home"
	routeCreate  = "/create"
	routeDestroy = "/destroy"
	routeOauth   = "/oauth2"
	routeHealth  = "/health"

	paramMessage = "message"
	paramName    = "name"

	cookieValidity = 10 * time.Minute
)

type param struct {
	key, value string
}

func makeURL(ctx *context.T, baseURL string, params ...param) string {
	u, err := url.Parse(baseURL)
	if err != nil {
		ctx.Errorf("Parse url error for %v: %v", baseURL, err)
		return ""
	}
	v := url.Values{}
	for _, p := range params {
		v.Add(p.key, p.value)
	}
	u.RawQuery = v.Encode()
	return u.String()
}

func replaceParam(ctx *context.T, origURL string, p param) string {
	u, err := url.Parse(origURL)
	if err != nil {
		ctx.Errorf("Parse url error for %v: %v", origURL, err)
		return ""
	}
	v := u.Query()
	v.Set(p.key, p.value)
	u.RawQuery = v.Encode()
	return u.String()
}

type httpArgs struct {
	addr,
	externalURL,
	serverName string
	secureCookies bool
	oauthCreds    *oauthCredentials
	baseBlessings security.Blessings
}

func (a httpArgs) validate() error {
	switch {
	case a.addr == "":
		return errors.New("addr is empty")
	case a.externalURL == "":
		return errors.New("externalURL is empty")
	}
	if err := a.oauthCreds.validate(); err != nil {
		return fmt.Errorf("oauth creds invalid: %v", err)
	}
	return nil
}

// startHTTP is the entry point to the http interface.  It configures and
// launches the http server.
func startHTTP(ctx *context.T, args httpArgs) {
	if err := args.validate(); err != nil {
		ctx.Fatalf("Invalid args %#v: %v", args, err)
	}
	baker := &signedCookieBaker{
		secure:   args.secureCookies,
		signKey:  args.oauthCreds.HashKey,
		validity: cookieValidity,
	}
	// mutating should be true for handlers that mutate state.  For such
	// handlers, any re-authentication should result in redirection to the
	// home page (to foil CSRF attacks that trick the user into launching
	// actions with consequences).
	newHandler := func(f handlerFunc, mutating bool) *handler {
		return &handler{
			ss: &serverState{
				ctx:  ctx,
				args: args,
			},
			baker:    baker,
			f:        f,
			mutating: mutating,
		}
	}

	http.HandleFunc(routeRoot, func(w http.ResponseWriter, r *http.Request) {
		tmplArgs := struct{ Home, ServerName string }{routeHome, args.serverName}
		if err := rootTmpl.Execute(w, tmplArgs); err != nil {
			errorOccurred(ctx, w, r, routeHome, err)
			ctx.Infof("%s[%s] : error %v", r.Method, r.URL, err)
		}
	})
	http.Handle(routeHome, newHandler(handleHome, false))
	http.Handle(routeCreate, newHandler(handleCreate, true))
	http.Handle(routeDestroy, newHandler(handleDestroy, true))
	http.HandleFunc(routeOauth, func(w http.ResponseWriter, r *http.Request) {
		handleOauth(ctx, args, baker, w, r)
	})
	http.HandleFunc(routeHealth, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	ctx.Infof("HTTP server at %v [%v]", args.addr, args.externalURL)
	go func() {
		if err := http.ListenAndServe(args.addr, nil); err != nil {
			ctx.Fatalf("ListenAndServe failed: %v", err)
		}
	}()
}

type serverState struct {
	ctx  *context.T
	args httpArgs
}

type requestState struct {
	email, csrfToken string
	w                http.ResponseWriter
	r                *http.Request
}

type handlerFunc func(ss *serverState, rs *requestState) error

// handler wraps handler functions and takes care of providing them with a
// Vanadium context, configuration args, and user's email address (performing
// the oauth flow if the user is not logged in yet).
type handler struct {
	ss       *serverState
	baker    cookieBaker
	f        handlerFunc
	mutating bool
}

// ServeHTTP verifies that the user is logged in, and redirects to the oauth
// flow if not.  If the user is logged in, it extracts the email address from
// the cookie and passes it to the handler function.
func (h handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := h.ss.ctx
	oauthCfg := oauthConfig(h.ss.args.externalURL, h.ss.args.oauthCreds)
	email, csrfToken, err := requireSession(ctx, oauthCfg, h.baker, w, r, h.mutating)
	if err != nil {
		errorOccurred(ctx, w, r, routeHome, err)
		ctx.Infof("%s[%s] : error %v", r.Method, r.URL, err)
		return
	}
	if email == "" {
		ctx.Infof("%s[%s] -> login", r.Method, r.URL)
		return
	}
	rs := &requestState{
		email:     email,
		csrfToken: csrfToken,
		w:         w,
		r:         r,
	}
	if err := h.f(h.ss, rs); err != nil {
		errorOccurred(ctx, w, r, makeURL(ctx, routeHome, param{paramCSRF, csrfToken}), err)
		ctx.Infof("%s[%s] : error %v", r.Method, r.URL, err)
		return
	}
	ctx.Infof("%s[%s] : OK", r.Method, r.URL)
}

// All the non-oauth handlers follow below.  The oauth handler is in oauth.go.

func handleHome(ss *serverState, rs *requestState) error {
	ctx := ss.ctx
	instances, err := list(ctx, rs.email)
	if err != nil {
		return fmt.Errorf("list error: %v", err)
	}
	type instanceArg struct{ Name, DestroyURL string }
	tmplArgs := struct {
		ServerName,
		Email,
		CreateURL,
		Message string
		Instances []instanceArg
	}{
		ServerName: ss.args.serverName,
		Email:      rs.email,
		CreateURL:  makeURL(ctx, routeCreate, param{paramCSRF, rs.csrfToken}),
		Message:    rs.r.FormValue(paramMessage),
	}
	for _, instance := range instances {
		tmplArgs.Instances = append(tmplArgs.Instances, instanceArg{
			Name:       instance,
			DestroyURL: makeURL(ctx, routeDestroy, param{paramName, instance}, param{paramCSRF, rs.csrfToken}),
		})
	}
	if err := homeTmpl.Execute(rs.w, tmplArgs); err != nil {
		return fmt.Errorf("failed to render home template: %v", err)
	}
	return nil
}

func handleCreate(ss *serverState, rs *requestState) error {
	ctx := ss.ctx
	name, err := create(ctx, rs.email, ss.args.baseBlessings)
	if err != nil {
		return fmt.Errorf("create failed: %v", err)
	}
	redirectTo := makeURL(ctx, routeHome, param{paramMessage, "created " + name}, param{paramCSRF, rs.csrfToken})
	http.Redirect(rs.w, rs.r, redirectTo, http.StatusFound)
	return nil
}

func handleDestroy(ss *serverState, rs *requestState) error {
	ctx := ss.ctx
	name := rs.r.FormValue(paramName)
	if err := destroy(ctx, rs.email, name); err != nil {
		return fmt.Errorf("destroy failed: %v", err)
	}
	redirectTo := makeURL(ctx, routeHome, param{paramMessage, "destroyed " + name}, param{paramCSRF, rs.csrfToken})
	http.Redirect(rs.w, rs.r, redirectTo, http.StatusFound)
	return nil
}
