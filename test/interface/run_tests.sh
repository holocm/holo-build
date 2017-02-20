#!/bin/bash

# find the directory containing the test cases
TESTS_DIR="$(readlink -f "$(dirname $0)")"

run_testcase() {
    local TEST_NAME="$1"
    echo ">> Running interface testcase $TEST_NAME..."

    # determine testcase location
    local TESTCASE_DIR="$TESTS_DIR/$TEST_NAME"
    if [ ! -d "$TESTCASE_DIR" ]; then
        echo "Cannot run $TEST_NAME: testcase not found" >&2
        return 1
    fi
    # set cwd!
    cd "$TESTCASE_DIR"
    # reset state of testcase directory
    rm -f -- stdout stderr *.deb *.pkg.tar.xz *.rpm

    # run test
    env INPUT_TOML=../input.toml \
        HOLO_BUILD=../../../build/holo-build \
        DUMP_PACKAGE=../../../build/dump-package \
        ./run.sh > stdout 2> stderr

    # strip ANSI colors from error output
    sed -i 's/\x1b\[[0-9;]*m//g' stderr

    # use diff to check the actual run with our expectations
    local EXIT_CODE=0
    for FILE in stdout stderr; do
        if [ -f $FILE ]; then
            if diff -q expected-$FILE $FILE >/dev/null; then true; else
                echo "!! The $FILE deviates from our expectation. Diff follows:"
                diff -u expected-$FILE $FILE 2>&1 | sed 's/^/    /'
                EXIT_CODE=1
            fi
        fi
    done
}

# this var will be set to 1 when a testcase fails
TEST_EXIT_CODE=0

# inspect arguments
if [ $# -gt 0 ]; then
    # testcase names given - run these testcases
    for TESTCASE in $@; do
        run_testcase $TESTCASE || TEST_EXIT_CODE=1
    done
else
    # no testcases given - run them all in order
    for TESTCASE in $(find "$TESTS_DIR" -mindepth 1 -maxdepth 1 -type d -printf '%f\n' | sort); do
        run_testcase $TESTCASE || TEST_EXIT_CODE=1
    done
fi

if [ $TEST_EXIT_CODE = 0 ]; then
    echo ">> All interface tests for holo-build completed successfully."
else
    echo "!! Some or all interface tests for holo-build failed. Please check the output above for more information."
fi
exit $TEST_EXIT_CODE
