// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"v.io/x/ref/test"
)

func TestAddPodAgent(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	const (
		myAppJSON = `{
  "apiVersion": "v1",
  "kind": "ReplicationController",
  "metadata": {
    "name": "my-app",
    "labels": {
      "run": "my-app"
    }
  },
  "spec": {
    "replicas": 5,
    "template": {
      "metadata": {
        "labels": {
          "run": "my-app"
        }
      },
      "spec": {
        "containers": [
          {
            "name": "my-app",
            "image": "registry/me/my-app:latest",
            "ports": [
              { "containerPort": 8193, "hostPort": 8193 }
            ],
            "volumeMounts": [
              { "name": "app-logs", "mountPath": "/logs" }
            ]
          }
        ],
        "volumes": [
          { "name": "app-logs", "emptyDir": {} }
        ]
      }
    }
  }
}`

		expected = `{
  "apiVersion": "v1",
  "kind": "ReplicationController",
  "metadata": {
    "labels": {
      "run": "my-app"
    },
    "name": "my-app"
  },
  "spec": {
    "replicas": 5,
    "template": {
      "metadata": {
        "labels": {
          "run": "my-app"
        }
      },
      "spec": {
        "containers": [
          {
            "env": [
              {
                "name": "V23_AGENT_PATH",
                "value": "/agent/socket/agent.sock"
              }
            ],
            "image": "registry/me/my-app:latest",
            "name": "my-app",
            "ports": [
              {
                "containerPort": 8193,
                "hostPort": 8193
              }
            ],
            "volumeMounts": [
              {
                "mountPath": "/logs",
                "name": "app-logs"
              },
              {
                "mountPath": "/agent/socket",
                "name": "agent-socket",
                "readOnly": true
              }
            ]
          },
          {
            "args": [
              "pod_agentd",
              "--agent=/(root/cluster-agent)@cluster-agent.test:8193",
              "--root-blessings=ROOT-BLESSINGS",
              "--secret-key-file=/agent/secret/secret",
              "--socket-path=/agent/socket/agent.sock",
              "--log_dir=/logs"
            ],
            "image": "",
            "name": "pod-agent",
            "volumeMounts": [
              {
                "mountPath": "/logs",
                "name": "agent-logs"
              },
              {
                "mountPath": "/agent/secret",
                "name": "agent-secret",
                "readOnly": true
              },
              {
                "mountPath": "/agent/socket",
                "name": "agent-socket"
              }
            ]
          }
        ],
        "volumes": [
          {
            "emptyDir": {},
            "name": "app-logs"
          },
          {
            "emptyDir": {},
            "name": "agent-logs"
          },
          {
            "name": "agent-secret",
            "secret": {
              "secretName": "myapp-secret"
            }
          },
          {
            "emptyDir": {},
            "name": "agent-socket"
          }
        ]
      }
    }
  }
}`
	)

	var myAppObj object
	if err := myAppObj.importJSON([]byte(myAppJSON)); err != nil {
		t.Fatalf("importJSON failed: %v", err)
	}

	config := &vkubeConfig{
		ClusterAgent: clusterAgentConfig{
			Blessing:  "root/cluster-agent",
			Namespace: "test",
		},
	}
	if err := addPodAgent(ctx, config, myAppObj, "myapp-secret"); err != nil {
		t.Fatalf("addPodAgent failed: %v", err)
	}
	outBytes, err := myAppObj.json()
	if err != nil {
		t.Fatalf("json failed: %v", err)
	}
	got := strings.Replace(string(outBytes), rootBlessings(ctx), "ROOT-BLESSINGS", 1)

	if got != expected {
		t.Errorf("unexpected output. Got %s, expected %s", got, expected)
		diff(t, expected, got)
	}
}

func diff(t *testing.T, expected, got string) {
	dir, err := ioutil.TempDir("", "diff-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	expectedFile := filepath.Join(dir, "expected")
	if err := ioutil.WriteFile(expectedFile, []byte(expected), 0644); err != nil {
		t.Fatal(err)
	}
	gotFile := filepath.Join(dir, "got")
	if err := ioutil.WriteFile(gotFile, []byte(got), 0644); err != nil {
		t.Fatal(err)
	}
	out, _ := exec.Command("diff", "-u", expectedFile, gotFile).CombinedOutput()
	t.Log(string(out))
}
