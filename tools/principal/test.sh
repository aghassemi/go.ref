#!/bin/bash

# Test the principal command-line tool.
#
# This tests most operations of the principal command-line tool.
# Not the "seekblessing" command yet, since that requires
# starting a separate server.

source "${VEYRON_ROOT}/scripts/lib/shell_test.sh"

readonly WORKDIR=$(shell::tmp_dir)

build() {
  veyron go build veyron.io/veyron/veyron/tools/principal || shell_test::fail "line ${LINENO}: failed to build principal"
}

# rmpublickey replaces public keys (16 hex bytes, :-separated) with XX:....
# This substitution enables comparison with golden output even when keys are freshly
# minted by the "principal create" command.
rmpublickey() {
    sed -e "s/\([0-9a-f]\{2\}:\)\{15\}[0-9a-f]\{2\}/XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX/g"
}

dumpblessings() {
    ./principal dumpblessings "$1" | rmpublickey
}

main() {
  cd "${WORKDIR}"
  build

  # Prevent any VEYRON_CREDENTIALS in the environment from interfering with this test.
  unset VEYRON_CREDENTIALS
  # Create two principals, one called "alice" one called "bob"
  ./principal create ./alice alice >/dev/null || shell_test::fail "line ${LINENO}: create failed"
  ./principal create ./bob bob >/dev/null || shell_test::fail "line ${LINENO}: create failed"
  # Run dump, bless, blessself on alice
  export VEYRON_CREDENTIALS=./alice
  ./principal blessself alicereborn >alice.blessself || shell_test::fail "line ${LINENO}: blessself failed"
  ./principal bless ./bob friend >alice.bless || shell_test::fail "line ${LINENO}: bless failed"
  ./principal dump >alice.dump || shell_test::fail "line ${LINENO}: dump failed"
  # Run store setdefault, store default, store set, store forpeer on bob
  export VEYRON_CREDENTIALS=./bob
  ./principal store setdefault alice.bless || shell_test::fail "line ${LINENO}: store setdefault failed"
  ./principal store default >bob.store.default || shell_test::fail "line ${LINENO}: store default failed"
  ./principal store set alice.bless alice/... || shell_test::fail "line ${LINENO}: store set failed"
  ./principal store forpeer alice/server >bob.store.forpeer || shell_test::fail "line ${LINENO}: store forpeer failed" 
  # Any other commands to be run without VEYRON_CREDENTIALS set.
  unset VEYRON_CREDENTIALS

  # Validate the output of various commands (mostly using "principal dumpblessings")
  cat alice.dump | rmpublickey >got || shell_test::fail "line ${LINENO}: cat alice.dump | rmpublickey failed"
  cat >want <<EOF
Public key : XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX
---------------- BlessingStore ----------------
Default blessings: alice(0 caveats)
Peer pattern                   : Blessings
...                            : alice(0 caveats)
---------------- BlessingRoots ----------------
Public key                                      : Pattern
XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX : [alice/...]
EOF
  if ! diff got want; then
  	shell_test::fail "line ${LINENO}"
  fi

  dumpblessings alice.blessself >got || shell_test::fail "line ${LINENO}: dumpblessings failed"
  cat >want <<EOF
Blessings          : alicereborn(0 caveats)
PublicKey          : XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX
Certificates       : 1 chains with (#certificates, #caveats) = (1, 0)
EOF
  if ! diff got want; then
  	shell_test::fail "line ${LINENO}"
  fi

  dumpblessings bob.store.default >got || shell_test::fail "line ${LINENO}: dumpblessings failed"
  cat >want <<EOF
Blessings          : alice/friend(1 caveats)
PublicKey          : XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX
Certificates       : 1 chains with (#certificates, #caveats) = (2, 1)
EOF
  if ! diff got want; then
	shell_test::fail "line ${LINENO}"
  fi

  dumpblessings bob.store.forpeer >got || shell_test::fail "line ${LINENO}: dumpblessings failed"
  cat >want <<EOF
Blessings          : bob(0 caveats)#alice/friend(1 caveats)
PublicKey          : XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX
Certificates       : 2 chains with (#certificates, #caveats) = (1, 0) + (2, 1)
EOF
  if ! diff got want; then
	shell_test::fail "line ${LINENO}"
  fi

  shell_test::pass
}

main "$@"
