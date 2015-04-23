// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package naming

import (
	"net"
	"reflect"
	"testing"

	"v.io/v23/naming"
)

func TestEndpoint(t *testing.T) {
	defver := defaultVersion
	defer func() {
		defaultVersion = defver
	}()
	v5a := &Endpoint{
		Protocol:     naming.UnknownProtocol,
		Address:      "batman.com:1234",
		RID:          naming.FixedRoutingID(0xdabbad00),
		IsMountTable: true,
	}
	v5b := &Endpoint{
		Protocol:     naming.UnknownProtocol,
		Address:      "batman.com:2345",
		RID:          naming.FixedRoutingID(0xdabbad00),
		IsMountTable: false,
	}
	v5c := &Endpoint{
		Protocol:     "tcp",
		Address:      "batman.com:2345",
		RID:          naming.FixedRoutingID(0x0),
		IsMountTable: false,
	}
	v5d := &Endpoint{
		Protocol:     "ws6",
		Address:      "batman.com:2345",
		RID:          naming.FixedRoutingID(0x0),
		IsMountTable: false,
	}
	v5e := &Endpoint{
		Protocol:     "tcp",
		Address:      "batman.com:2345",
		RID:          naming.FixedRoutingID(0xba77),
		IsMountTable: true,
		Blessings:    []string{"dev.v.io/foo@bar.com", "dev.v.io/bar@bar.com/delegate"},
	}
	v5f := &Endpoint{
		Protocol:     "tcp",
		Address:      "batman.com:2345",
		RID:          naming.FixedRoutingID(0xba77),
		IsMountTable: true,
		// Blessings that look similar to other parts of the endpoint.
		Blessings: []string{"@@", "@s", "@m"},
	}

	testcasesA := []struct {
		endpoint naming.Endpoint
		address  string
	}{
		{v5a, "batman.com:1234"},
		{v5b, "batman.com:2345"},
		{v5c, "batman.com:2345"},
	}
	for _, test := range testcasesA {
		addr := test.endpoint.Addr()
		if addr.String() != test.address {
			t.Errorf("unexpected address %q, not %q", addr.String(), test.address)
		}
	}

	// Test v5 endpoints.
	testcasesC := []struct {
		Endpoint naming.Endpoint
		String   string
		Version  int
	}{
		{v5a, "@5@@batman.com:1234@000000000000000000000000dabbad00@m@@@", 5},
		{v5b, "@5@@batman.com:2345@000000000000000000000000dabbad00@s@@@", 5},
		{v5c, "@5@tcp@batman.com:2345@00000000000000000000000000000000@s@@@", 5},
		{v5d, "@5@ws6@batman.com:2345@00000000000000000000000000000000@s@@@", 5},
		{v5e, "@5@tcp@batman.com:2345@0000000000000000000000000000ba77@m@dev.v.io/foo@bar.com,dev.v.io/bar@bar.com/delegate@@", 5},
		{v5f, "@5@tcp@batman.com:2345@0000000000000000000000000000ba77@m@@@,@s,@m@@", 5},
	}

	for i, test := range testcasesC {
		if got, want := test.Endpoint.VersionedString(test.Version), test.String; got != want {
			t.Errorf("Test %d: Got %q want %q for endpoint (v%d): %#v", i, got, want, test.Version, test.Endpoint)
		}
		ep, err := NewEndpoint(test.String)
		if err != nil {
			t.Errorf("Test %d: Endpoint(%q) failed with %v", i, test.String, err)
			continue
		}
		if !reflect.DeepEqual(ep, test.Endpoint) {
			t.Errorf("Test %d: Got endpoint %#v, want %#v for string %q", i, ep, test.Endpoint, test.String)
		}
	}
}

type endpointTest struct {
	input, output string
	err           error
}

func runEndpointTests(t *testing.T, testcases []endpointTest) {
	for _, test := range testcases {
		ep, err := NewEndpoint(test.input)
		if err == nil && test.err == nil && ep.String() != test.output {
			t.Errorf("NewEndpoint(%q): unexpected endpoint string %q != %q",
				test.input, ep.String(), test.output)
			continue
		}
		switch {
		case test.err == err: // do nothing
		case test.err == nil && err != nil:
			t.Errorf("NewEndpoint(%q): unexpected error %q", test.output, err)
		case test.err != nil && err == nil:
			t.Errorf("NewEndpoint(%q): missing error %q", test.output, test.err)
		case err.Error() != test.err.Error():
			t.Errorf("NewEndpoint(%q): unexpected error  %q != %q", test.output, err, test.err)
		}
	}
}

func TestHostPortEndpoint(t *testing.T) {
	defver := defaultVersion
	defer func() {
		defaultVersion = defver
	}()
	testcases := []endpointTest{
		{"localhost:10", "@5@@localhost:10@00000000000000000000000000000000@m@@@", nil},
		{"localhost:", "@5@@localhost:@00000000000000000000000000000000@m@@@", nil},
		{"localhost", "", errInvalidEndpointString},
		{"(dev.v.io/service/mounttabled)@ns.dev.v.io:8101", "@5@@ns.dev.v.io:8101@00000000000000000000000000000000@m@dev.v.io/service/mounttabled@@", nil},
		{"(dev.v.io/users/foo@bar.com)@ns.dev.v.io:8101", "@5@@ns.dev.v.io:8101@00000000000000000000000000000000@m@dev.v.io/users/foo@bar.com@@", nil},
		{"(@1@tcp)@ns.dev.v.io:8101", "@5@@ns.dev.v.io:8101@00000000000000000000000000000000@m@@1@tcp@@", nil},
	}
	runEndpointTests(t, testcases)
}

func TestParseHostPort(t *testing.T) {
	dns := &Endpoint{
		Protocol:     "tcp",
		Address:      "batman.com:4444",
		IsMountTable: true,
	}
	ipv4 := &Endpoint{
		Protocol:     "tcp",
		Address:      "192.168.1.1:4444",
		IsMountTable: true,
	}
	ipv6 := &Endpoint{
		Protocol:     "tcp",
		Address:      "[01:02::]:4444",
		IsMountTable: true,
	}
	testcases := []struct {
		Endpoint   naming.Endpoint
		Host, Port string
	}{
		{dns, "batman.com", "4444"},
		{ipv4, "192.168.1.1", "4444"},
		{ipv6, "01:02::", "4444"},
	}

	for _, test := range testcases {
		addr := net.JoinHostPort(test.Host, test.Port)
		epString := naming.FormatEndpoint("tcp", addr)
		if ep, err := NewEndpoint(epString); err != nil {
			t.Errorf("NewEndpoint(%q) failed with %v", addr, err)
		} else {
			if !reflect.DeepEqual(test.Endpoint, ep) {
				t.Errorf("Got endpoint %T = %#v, want %T = %#v for string %q", ep, ep, test.Endpoint, test.Endpoint, addr)
			}
		}
	}
}
