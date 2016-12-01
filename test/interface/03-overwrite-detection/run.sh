#!/bin/sh

# check that a rebuild of the same input.toml does not touch the file
#
# To do so, the mtime of package.deb is set to 0 (i.e. 1970-01-01T00:00:00Z)
# before the rebuild. If package.deb is written during the rebuild, the `find`
# call should list it.

echo checking successful rebuild
echo checking successful rebuild >&2
${HOLO_BUILD} --format=debian -o package.deb ${INPUT_TOML}
touch --date='@0' package.deb
${HOLO_BUILD} --format=debian -o package.deb ${INPUT_TOML}
find . -name \*.deb -mtime 0 # should output nothing

# same test with --force, this time we should see a rewrite

echo checking forceful rebuild
echo checking forceful rebuild >&2
${HOLO_BUILD} --force --format=debian -o package.deb ${INPUT_TOML}
touch --date='@0' package.deb
${HOLO_BUILD} --force --format=debian -o package.deb ${INPUT_TOML}
find . -name \*.deb -mtime 0 # should output "package.deb"

# check that a rebuild fails if the package contents change
#
# The rationale is that, when the package declaration changes, the version or
# release should be bumped, thus resulting in a different filename and thus no
# collision with an existing package file.
#
# To simulate a rebuild with different package declarations, we just write some
# garbage into package.deb first.

echo checking rebuild with error
echo checking rebuild with error >&2
dd if=/dev/zero of=package.deb bs=4K count=1 status=none
${HOLO_BUILD} --format=debian -o package.deb ${INPUT_TOML}
file package.deb

# same test with --force, should overwrite without complaining

echo checking forceful rebuild without error
echo checking forceful rebuild without error >&2
dd if=/dev/zero of=package.deb bs=4K count=1 status=none
${HOLO_BUILD} --force --format=debian -o package.deb ${INPUT_TOML}
file package.deb
