// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"v.io/v23"
	"v.io/v23/security"
)

func handleHome(ss *serverState, rs *requestState) error {
	ctx := ss.ctx
	instances, err := list(ctx, rs.email)
	if err != nil {
		return fmt.Errorf("list error: %v", err)
	}
	type instanceArg struct {
		Name,
		NameRoot,
		DestroyURL,
		DashboardURL string
		BlessingPatterns []string
		CreationTime     time.Time
	}
	tmplArgs := struct {
		AssetsPrefix,
		ServerName,
		Email,
		CreateURL,
		Message string
		Instances []instanceArg
	}{
		AssetsPrefix: ss.args.staticAssetsPrefix,
		ServerName:   ss.args.serverName,
		Email:        rs.email,
		CreateURL:    makeURL(ctx, routeCreate, params{paramCSRF: rs.csrfToken}),
		Message:      rs.r.FormValue(paramMessage),
	}
	for _, instance := range instances {
		var patterns []string
		for _, b := range security.BlessingNames(v23.GetPrincipal(ctx), ss.args.baseBlessings) {
			bName := strings.Join([]string{b, instance.name}, security.ChainSeparator)
			patterns = append(patterns, bName)
		}

		tmplArgs.Instances = append(tmplArgs.Instances, instanceArg{
			Name:             instance.mountName,
			NameRoot:         nameRoot(ctx),
			CreationTime:     instance.creationTime,
			BlessingPatterns: patterns,
			DestroyURL:       makeURL(ctx, routeDestroy, params{paramName: instance.mountName, paramCSRF: rs.csrfToken}),
			DashboardURL:     makeURL(ctx, routeDashboard, params{paramDashboardName: relativeMountName(instance.mountName), paramCSRF: rs.csrfToken}),
		})
	}
	if err := ss.args.assets.executeTemplate(rs.w, homeTmpl, tmplArgs); err != nil {
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
	redirectTo := makeURL(ctx, routeHome, params{paramMessage: "created " + name, paramCSRF: rs.csrfToken})
	http.Redirect(rs.w, rs.r, redirectTo, http.StatusFound)
	return nil
}

func handleDestroy(ss *serverState, rs *requestState) error {
	ctx := ss.ctx
	name := rs.r.FormValue(paramName)
	if err := destroy(ctx, rs.email, name); err != nil {
		return fmt.Errorf("destroy failed: %v", err)
	}
	redirectTo := makeURL(ctx, routeHome, params{paramMessage: "destroyed " + name, paramCSRF: rs.csrfToken})
	http.Redirect(rs.w, rs.r, redirectTo, http.StatusFound)
	return nil
}
