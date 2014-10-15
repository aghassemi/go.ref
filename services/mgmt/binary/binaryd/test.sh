#!/bin/bash

# Test the binary repository daemon.
#
# This test starts a binary repository daemon and uses the binary
# repository client to verify that <binary>.Upload(),
# <binary>.Download(), and <binary>.Delete() work as expected.

source "${VEYRON_ROOT}/scripts/lib/shell_test.sh"

build() {
  veyron go build veyron.io/veyron/veyron/services/mgmt/binary/binaryd || shell_test::fail "line ${LINENO}: failed to build 'binaryd'"
  veyron go build veyron.io/veyron/veyron/tools/binary || shell_test::fail "line ${LINENO}: failed to build 'binary'"
}

main() {
  cd "${TMPDIR}"
  build

  shell_test::setup_server_test

  # Start the binary repository daemon.
  local -r REPO="binaryd-test-repo"
  shell_test::start_server ./binaryd --name="${REPO}" --veyron.tcp.address=127.0.0.1:0 \
    || shell_test::fail "line ${LINENO} failed to start binaryd"

  # Create a binary file.
  local -r BINARY="${REPO}/test-binary"
  local -r BINARY_FILE=$(shell::tmp_file)
  dd if=/dev/urandom of="${BINARY_FILE}" bs=1000000 count=16 \
    || shell_test::fail "line ${LINENO}: faile to create a random binary file"
  ./binary upload "${BINARY}" "${BINARY_FILE}" || shell_test::fail "line ${LINENO}: 'upload' failed"

  # Download the binary file.
  local -r BINARY_FILE2=$(shell::tmp_file)
  ./binary download "${BINARY}" "${BINARY_FILE2}" || shell_test::fail "line ${LINENO}: 'download' failed"
  if [[ $(cmp "${BINARY_FILE}" "${BINARY_FILE2}" &> /dev/null) ]]; then
    shell_test::fail "mismatching binary files"
  fi

  # Remove the binary file.
  ./binary delete "${BINARY}" || shell_test::fail "line ${LINENO}: 'delete' failed"

  # Check the binary no longer exists.
  local -r RESULT=$(shell::check_result ./binary download "${BINARY}" "${BINARY_FILE2}")
  shell_test::assert_ne "${RESULT}" "0" "${LINENO}"

  shell_test::pass
}

main "$@"
