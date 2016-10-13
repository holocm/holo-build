#!/bin/sh

# check that --reproducible and --no-reproducible produce the same output

${HOLO_BUILD} -o package1.deb --format=debian --reproducible    ${INPUT_TOML}
${HOLO_BUILD} -o package2.deb --format=debian --no-reproducible ${INPUT_TOML}

file package1.deb
file package2.deb
diff package1.deb package2.deb # should produce no output
