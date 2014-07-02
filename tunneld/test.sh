#!/bin/sh

# Test the tunneld binary
#
# This test starts a tunnel server and a mounttable server and then verifies
# that vsh can run commands through it and that all the expected names are
# in the mounttable.

toplevel=$(git rev-parse --show-toplevel)
go=${toplevel}/scripts/build/go
thisscript=$0

echo "Test directory: $(dirname $0)"

builddir=$(mktemp -d --tmpdir=${toplevel}/go)
trap onexit EXIT

onexit() {
	cd /
	exec 2> /dev/null
	kill -9 $(jobs -p)
	rm -rf $builddir
}

FAIL() {
	[ $# -gt 0 ] && echo "$thisscript $*"
	echo FAIL
	exit 1
}

PASS() {
	echo PASS
	exit 0
}

# Build binaries.
cd $builddir
$go build veyron/examples/tunnel/tunneld || FAIL "line $LINENO: failed to build tunneld"
$go build veyron/examples/tunnel/vsh || FAIL "line $LINENO: failed to build vsh"
$go build veyron/services/mounttable/mounttabled || FAIL "line $LINENO: failed to build mounttabled"
$go build veyron/tools/mounttable || FAIL "line $LINENO: failed to build mounttable"
$go build veyron/tools/identity || FAIL "line $LINENO: failed to build identity"

# Start mounttabled and find its endpoint.
mtlog=$(mktemp --tmpdir=.)
./mounttabled --address=localhost:0 > $mtlog 2>&1 &

for i in 1 2 3 4; do
	ep=$(grep "Mount table service at:" $mtlog | sed -re 's/^.*endpoint: ([^ ]*).*/\1/')
	if [ -n "$ep" ]; then
		break
	fi
	sleep 1
done
[ -z $ep ] && FAIL "line $LINENO: no mounttable server"

tmpid=$(mktemp --tmpdir=.)
./identity --name=test > $tmpid

export NAMESPACE_ROOT=$ep
export VEYRON_IDENTITY=$tmpid

# Start tunneld and find its endpoint.
tunlog=$(mktemp --tmpdir=.)
./tunneld --address=localhost:0 > $tunlog 2>&1 &

for i in 1 2 3 4; do
	ep=$(grep "Listening on endpoint" $tunlog | sed -re 's/^.*endpoint ([^ ]*).*/\1/')
	if [ -n "$ep" ]; then
		break
	fi
	sleep 1
done
[ -z $ep ] && FAIL "line $LINENO: no tunnel server"

# Run remote command with the endpoint.
got=$(./vsh $ep echo HELLO WORLD)
want="HELLO WORLD"

if [ "$got" != "$want" ]; then
	FAIL "line $LINENO: unexpected output. Got $got, want $want"
fi

# Run remote command with the object name.
got=$(./vsh tunnel/id/test echo HELLO WORLD)
want="HELLO WORLD"

if [ "$got" != "$want" ]; then
	FAIL "line $LINENO: unexpected output. Got $got, want $want"
fi

# Verify that all the published names are there.
got=$(./mounttable glob $NAMESPACE_ROOT 'tunnel/*/*' |    \
      sed -e 's/TTL .m..s/TTL XmXXs/'                     \
          -e 's!hwaddr/[^ ]*!hwaddr/XX:XX:XX:XX:XX:XX!' | \
      sort)
want="[$NAMESPACE_ROOT]
tunnel/hostname/$(hostname) $ep// (TTL XmXXs)
tunnel/hwaddr/XX:XX:XX:XX:XX:XX $ep// (TTL XmXXs)
tunnel/id/test $ep// (TTL XmXXs)"

if [ "$got" != "$want" ]; then
	FAIL "line $LINENO: unexpected output. Got $got, want $want"
fi

PASS
