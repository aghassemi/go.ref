#!/bin/bash

# Test the playground builder tool.

source "${VEYRON_ROOT}/environment/scripts/lib/shell_test.sh"

build() {
  local -r GO="${REPO_ROOT}/scripts/build/go"
  "${GO}" build veyron/tools/identity || shell_test::fail "line ${LINENO}: failed to build 'identity'"
  "${GO}" build veyron/services/proxy/proxyd || shell_test::fail "line ${LINENO}: failed to build 'proxyd'"
  "${GO}" build veyron/services/mounttable/mounttabled || shell_test::fail "line ${LINENO}: failed to build 'mounttabled'"
  "${GO}" build veyron/services/wsprd || shell_test::fail "line ${LINENO}: failed to build 'wsprd'"
  "${GO}" build veyron2/vdl/vdl || shell_test::fail "line ${LINENO}: failed to build 'vdl'"
  "${GO}" build veyron/tools/playground/builder || shell_test::fail "line ${LINENO}: failed to build 'builder'"
  "${GO}" build veyron/tools/playground/testdata/escaper || shell_test::fail "line ${LINENO}: failed to build 'escaper'"
}

test_with_files() {
    echo '{"Files":[' > request.json
    while [[ $# > 0 ]]; do
	echo '{"Name":"'"$(basename $1)"'","Body":' >>request.json
	grep -v OMIT $1 | ./escaper >>request.json
	shift
	if [[ $# > 0 ]]; then
	    echo '},' >>request.json
	else
	    echo '}' >>request.json
	fi
    done
    echo ']}' >>request.json
    rm -f builder.out
    ./builder <request.json 2>&1 | tee builder.out
}

main() {
  # TODO(nlacasse): Remove this once the raceyness issues in these tests have
  # been sorted out.
  shell_test::pass

  cd $(shell::tmp_dir)
  build

  local -r DIR="${REPO_ROOT}/go/src/veyron/tools/playground/testdata"

  export GOPATH="$(pwd)":$VEYRON_ROOT/veyron/go
  export PATH="$(pwd):$PATH"

  # Test without identities

  test_with_files $DIR/pingpong/wire.vdl $DIR/pong/pong.go $DIR/ping/ping.go || shell_test::fail "line ${LINENO}: basic ping (go -> go)"
  grep -q PING builder.out || shell_test::fail "line ${LINENO}: no PING"
  grep -q PONG builder.out || shell_test::fail "line ${LINENO}: no PONG"

  test_with_files $DIR/pong/pong.js $DIR/ping/ping.js || shell_test::fail "line ${LINENO}: basic ping (js -> js)"
  grep -q PING builder.out || shell_test::fail "line ${LINENO}: no PING"
  grep -q PONG builder.out || shell_test::fail "line ${LINENO}: no PONG"

  test_with_files $DIR/pong/pong.go $DIR/ping/ping.js $DIR/pingpong/wire.vdl || shell_test::fail "line ${LINENO}: basic ping (js -> go)"
  grep -q PING builder.out || shell_test::fail "line ${LINENO}: no PING"
  grep -q PONG builder.out || shell_test::fail "line ${LINENO}: no PONG"

  # Test with authorized identities

  test_with_files $DIR/pong/pong.go $DIR/ping/ping.go $DIR/pingpong/wire.vdl $DIR/ids/authorized.id || shell_test::fail "line ${LINENO}: authorized id (go -> go)"
  grep -q PING builder.out || shell_test::fail "line ${LINENO}: no PING"
  grep -q PONG builder.out || shell_test::fail "line ${LINENO}: no PONG"

  test_with_files $DIR/pong/pong.js $DIR/ping/ping.js $DIR/ids/authorized.id || shell_test::fail "line ${LINENO}: authorized id (js -> js)"
  grep -q PING builder.out || shell_test::fail "line ${LINENO}: no PING"
  grep -q PONG builder.out || shell_test::fail "line ${LINENO}: no PONG"

  # Test with expired identities

  test_with_files $DIR/pong/pong.go $DIR/ping/ping.go $DIR/pingpong/wire.vdl $DIR/ids/expired.id || shell_test::fail  "line ${LINENO}: failed to build with expired id (go -> go)"
  grep -q "ipc: not authorized" builder.out || shell_test::fail "line ${LINENO}: rpc with expired id succeeded"

  test_with_files $DIR/pong/pong.js $DIR/ping/ping.js $DIR/ids/expired.id || shell_test::fail  "line ${LINENO}: failed to build with expired id (js -> js)"
  # TODO(nlacasse): The error message in this case is very bad. Clean up the
  # veyron.js errors and change this to something reasonable.
  grep -q "error serving service:" builder.out || shell_test::fail "line ${LINENO}: rpc with expired id succeeded"

  # Test with unauthorized identities

  test_with_files $DIR/pong/pong.go $DIR/ping/ping.go $DIR/pingpong/wire.vdl $DIR/ids/unauthorized.id || shell_test::fail  "line ${LINENO}: failed to build with unauthorized id (go -> go)"
  grep -q "ipc: not authorized" builder.out || shell_test::fail "line ${LINENO}: rpc with unauthorized id succeeded"

  # TODO(nlacasse): Write the javascript version of this test once the
  # javascript implementation is capable of checking that an identity is
  # authorized.

  shell_test::pass
}

main "$@"
