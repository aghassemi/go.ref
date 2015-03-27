// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package exec

import (
	"strings"

	"v.io/v23/verror"
)

var (
	errNotFound = verror.Register(pkgPath+".errNotFound", verror.NoRetry, "{1:}{2:} not found{:_}")
)

// Getenv retrieves the value of the given variable from the given
// slice of environment variable assignments.
func Getenv(env []string, name string) (string, error) {
	for _, v := range env {
		if strings.HasPrefix(v, name+"=") {
			return strings.TrimPrefix(v, name+"="), nil
		}
	}
	return "", verror.New(errNotFound, nil)
}

// Setenv updates / adds the value assignment for the given variable
// in the given slice of environment variable assigments.
func Setenv(env []string, name, value string) []string {
	newValue := name + "=" + value
	for i, v := range env {
		if strings.HasPrefix(v, name+"=") {
			env[i] = newValue
			return env
		}
	}
	return append(env, newValue)
}

// Mergeenv merges the values for the variables contained in 'other' with the
// values contained in 'base'.  If a variable exists in both, the value in
// 'other' takes precedence.
func Mergeenv(base, other []string) []string {
	otherValues := make(map[string]string)
	otherUsed := make(map[string]bool)
	for _, v := range other {
		if parts := strings.SplitN(v, "=", 2); len(parts) == 2 {
			otherValues[parts[0]] = parts[1]
		}
	}
	for i, v := range base {
		if parts := strings.SplitN(v, "=", 2); len(parts) == 2 {
			if otherValue, ok := otherValues[parts[0]]; ok {
				base[i] = parts[0] + "=" + otherValue
				otherUsed[parts[0]] = true
			}
		}
	}
	for k, v := range otherValues {
		if !otherUsed[k] {
			base = append(base, k+"="+v)
		}
	}
	return base
}
