#!/bin/sh

# check that --reproducible and --no-reproducible produce the same output

${HOLO_BUILD} --stdout --debian --reproducible    ${INPUT_TOML} > package1.deb
${HOLO_BUILD} --stdout --debian --no-reproducible ${INPUT_TOML} > package2.deb
diff package1.deb package2.deb # should produce no output
