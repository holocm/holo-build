#!/bin/sh

# check that if "-o" is set to a directory, then the package will be placed
# there with the recommended basename

echo checking existing output directory
echo checking existing output directory >&2
${HOLO_BUILD} --format=debian -o foo/bar ${INPUT_TOML}
file foo/bar/package_1.0-1_all.deb

echo checking missing output directory
echo checking missing output directory >&2
${HOLO_BUILD} --format=debian -o does/not/exist ${INPUT_TOML}
