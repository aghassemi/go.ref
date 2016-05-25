// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"text/template"
	"time"

	"v.io/v23/context"
	"v.io/v23/conventions"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/security/access"
	"v.io/v23/verror"
	"v.io/v23/vom"
)

const pkgPath = "v.io/x/ref/services/allocator/allocatord"

var (
	errLimitExceeded       = verror.Register(pkgPath+".errLimitExceeded", verror.NoRetry, "{1:}{2:} limit ({3}) exceeded")
	errGlobalLimitExceeded = verror.Register(pkgPath+".errGlobalLimitExceeded", verror.NoRetry, "{1:}{2:} global limit ({3}) exceeded")
)

type allocatorImpl struct {
	baseBlessings security.Blessings
}

// Create creates a new instance of the service.
// It returns the object name of the new instance.
func (i *allocatorImpl) Create(ctx *context.T, call rpc.ServerCall) (string, error) {
	b, _ := security.RemoteBlessingNames(ctx, call.Security())
	ctx.Infof("Create() called by %v", b)

	email := emailFromBlessingNames(b)
	if email == "" {
		return "", verror.New(verror.ErrNoAccess, ctx, "unable to determine caller's email address")
	}
	return create(ctx, email, i.baseBlessings)
}

// Destroy destroys the instance with the given name.
func (i *allocatorImpl) Destroy(ctx *context.T, call rpc.ServerCall, mName string) error {
	b, _ := security.RemoteBlessingNames(ctx, call.Security())
	ctx.Infof("Destroy(%q) called by %v", mName, b)

	email := emailFromBlessingNames(b)
	if email == "" {
		return verror.New(verror.ErrNoAccess, ctx, "unable to determine caller's email address")
	}
	return destroy(ctx, email, mName)
}

// List returns a list of all the instances owned by the caller.
func (i *allocatorImpl) List(ctx *context.T, call rpc.ServerCall) ([]string, error) {
	b, _ := security.RemoteBlessingNames(ctx, call.Security())
	ctx.Infof("List() called by %v", b)

	email := emailFromBlessingNames(b)
	if email == "" {
		return nil, verror.New(verror.ErrNoAccess, ctx, "unable to determine caller's email address")
	}
	instances, err := list(ctx, email)
	if err != nil {
		return nil, err
	}
	mNames := make([]string, len(instances))
	for i, instance := range instances {
		mNames[i] = instance.mountName
	}
	return mNames, nil
}

func create(ctx *context.T, email string, baseBlessings security.Blessings) (string, error) {
	// Enforce a limit on the number of instances. These tests are a little
	// bit racy. It's possible that multiple calls to create() will run
	// concurrently and that we'll end up with too many instances.
	if n, err := serverInstances(email); err != nil {
		return "", err
	} else if len(n) >= maxInstancesPerUserFlag {
		return "", verror.New(errLimitExceeded, ctx, maxInstancesPerUserFlag)
	}

	if n, err := serverInstances(""); err != nil {
		return "", err
	} else if len(n) >= maxInstancesFlag {
		return "", verror.New(errGlobalLimitExceeded, ctx, maxInstancesFlag)
	}

	kName, err := newKubeName()
	if err != nil {
		return "", err
	}
	mName := mountNameFromKubeName(ctx, kName)

	cfg, cleanup, err := createDeploymentConfig(ctx, email, kName, mName)
	defer cleanup()
	if err != nil {
		return "", err
	}

	vomBlessings, err := vom.Encode(baseBlessings)
	if err != nil {
		return "", err
	}

	if err := createPersistentDisk(ctx, kName); err != nil {
		return "", err
	}

	out, err := vkube(
		"start", "-f", cfg,
		"--base-blessings", base64.URLEncoding.EncodeToString(vomBlessings),
		"--wait",
		kName,
	)
	if err != nil {
		ctx.Errorf("vkube start failed: %s", string(out))
		deletePersistentDisk(ctx, kName)
		return "", verror.New(verror.ErrInternal, ctx, err)
	}
	return mName, nil
}

func destroy(ctx *context.T, email, mName string) error {
	kName := kubeNameFromMountName(mName)

	if err := isOwnerOfInstance(email, kName); err != nil {
		return err
	}

	cfg, cleanup, err := createDeploymentConfig(ctx, email, kName, mName)
	defer cleanup()
	if err != nil {
		return err
	}

	out, err := vkube("stop", "-f", cfg)
	if err != nil {
		ctx.Errorf("vkube stop failed: %s", string(out))
		return verror.New(verror.ErrInternal, ctx, err)
	}
	if err := deletePersistentDisk(ctx, kName); err != nil {
		return err
	}
	return nil
}

func list(ctx *context.T, email string) ([]serverInstance, error) {
	instances, err := serverInstances(email)
	if err != nil {
		return nil, err
	}
	for i, instance := range instances {
		instances[i].mountName = mountNameFromKubeName(ctx, instance.name)
	}
	return instances, err
}

func createDeploymentConfig(ctx *context.T, email, deploymentName, mountName string) (string, func(), error) {
	cleanup := func() {}
	acl, err := accessList(ctx, email)
	if err != nil {
		return "", cleanup, err
	}
	creatorInfo, err := creatorInfo(ctx, email)
	if err != nil {
		return "", cleanup, err
	}

	t, err := template.ParseFiles(deploymentTemplateFlag)
	if err != nil {
		return "", cleanup, err
	}
	data := struct {
		AccessList  string
		CreatorInfo string
		MountName   string
		Name        string
		OwnerHash   string
	}{
		AccessList:  acl,
		CreatorInfo: creatorInfo,
		MountName:   mountName,
		Name:        deploymentName,
		OwnerHash:   emailHash(email),
	}

	f, err := ioutil.TempFile("", "allocator-deployment-")
	if err != nil {
		return "", cleanup, err
	}
	defer f.Close()
	cleanup = func() { os.Remove(f.Name()) }

	if err := t.Execute(f, data); err != nil {
		return "", cleanup, err
	}
	return f.Name(), cleanup, nil
}

// accessList returns a double encoded JSON access list that can be used in a
// Deployment template that contains something like:
//   "--v23.permissions.literal={\"Admin\": {{.AccessList}} }"
// The access list include the creator.
func accessList(ctx *context.T, email string) (string, error) {
	var acl access.AccessList
	if globalAdminsFlag != "" {
		for _, admin := range strings.Split(globalAdminsFlag, ",") {
			acl.In = append(acl.In, security.BlessingPattern(admin))
		}
	}
	for _, blessing := range conventions.ParseBlessingNames(blessingNamesFromEmail(email)...) {
		acl.In = append(acl.In, blessing.UserPattern())
	}
	j, err := json.Marshal(acl)
	if err != nil {
		ctx.Errorf("json.Marshal(%#v) failed: %v", acl, err)
		return "", err
	}
	// JSON encode again, because the access list is in a JSON template.
	str := string(j)
	j, err = json.Marshal(str)
	if err != nil {
		ctx.Errorf("json.Marshal(%#v) failed: %v", str, err)
		return "", err
	}
	// Remove the quotes.
	return string(j[1 : len(j)-1]), nil
}

// creatorInfo returns a double encoded JSON access list that can be used as
// annotation in a Deployment template, e.g.
//   "annotations": {
//     "v.io/allocatord/creator-info": {{.CreatorInfo}}
//   }
func creatorInfo(ctx *context.T, email string) (string, error) {
	info := struct {
		Email string `json:"email"`
	}{email}
	j, err := json.Marshal(info)
	if err != nil {
		ctx.Errorf("json.Marshal(%#v) failed: %v", info, err)
		return "", err
	}
	// JSON encode again, because the annotation is in a JSON template.
	str := string(j)
	j, err = json.Marshal(str)
	if err != nil {
		ctx.Errorf("json.Marshal(%#v) failed: %v", str, err)
		return "", err
	}
	return string(j), nil
}

func emailHash(email string) string {
	h := sha1.Sum([]byte(email))
	return hex.EncodeToString(h[:])
}

type serverInstance struct {
	name         string
	mountName    string
	creationTime time.Time
}

func serverInstances(email string) ([]serverInstance, error) {
	// TODO(caprita): Store the mount name and server blessing name(s) as
	// annotations in the deployment, and retrieve them here.  This should
	// ensure that the values are accurate, and do not depend on the version
	// of allocatord that is presenting them.
	args := []string{"kubectl", "get", "deployments", "-o", "json"}
	if email != "" {
		args = append(args, "-l", "ownerHash="+emailHash(email))
	}

	var out []byte
	var err error
	for retry := 0; retry < 10; retry++ {
		if out, err = vkube(args...); err == nil {
			break
		}
		time.Sleep(time.Second)
	}
	if err != nil {
		return nil, err
	}

	var list struct {
		Items []struct {
			Metadata struct {
				Name         string    `json:"name"`
				CreationTime time.Time `json:"creationTimestamp"`
			} `json:"metadata"`
		} `json:"items"`
	}
	if err := json.Unmarshal(out, &list); err != nil {
		return nil, err
	}
	instances := []serverInstance{}
	for _, l := range list.Items {
		if strings.HasPrefix(l.Metadata.Name, serverNameFlag+"-") {
			instances = append(instances, serverInstance{
				name:         l.Metadata.Name,
				creationTime: l.Metadata.CreationTime,
			})
		}
	}
	return instances, nil
}

func isOwnerOfInstance(email, kName string) error {
	instances, err := serverInstances(email)
	if err != nil {
		return err
	}
	for _, i := range instances {
		if i.name == kName {
			return nil
		}
	}
	return verror.New(verror.ErrNoExistOrNoAccess, nil)
}

func createPersistentDisk(ctx *context.T, name string) error {
	if out, err := gcloud("compute", "disks", "create", name, "--size", diskSizeFlag); err != nil {
		ctx.Errorf("disk creation failed: %v: %s", err, string(out))
		return err
	}
	return nil
}

func deletePersistentDisk(ctx *context.T, name string) error {
	var (
		start = time.Now()
		out   []byte
		err   error
	)
	for time.Since(start) < 5*time.Minute {
		if out, err = gcloud("compute", "disks", "delete", name); err == nil {
			return nil
		}
		time.Sleep(time.Second)
	}
	ctx.Errorf("disk deletion failed: %v: %s", err, string(out))
	return err
}

func gcloud(args ...string) ([]byte, error) {
	data, err := ioutil.ReadFile(vkubeCfgFlag)
	if err != nil {
		return nil, err
	}
	var config struct {
		Project string `json:"project"`
		Zone    string `json:"zone"`
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	args = append(args, "--project", config.Project, "--zone", config.Zone)
	return exec.Command(gcloudBinFlag, args...).CombinedOutput()
}

func vkube(args ...string) ([]byte, error) {
	args = append(
		[]string{
			"--config=" + vkubeCfgFlag,
			"--no-headers",
		},
		args...,
	)
	return exec.Command(vkubeBinFlag, args...).CombinedOutput()
}
