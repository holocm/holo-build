#!/bin/sh

# check that --reproducible and --no-reproducible produce the same output

echo building with --reproducible >&2
${HOLO_BUILD} -o package1.deb --format=debian --reproducible    ${INPUT_TOML}
echo building with --no-reproducible >&2
${HOLO_BUILD} -o package2.deb --format=debian --no-reproducible ${INPUT_TOML}

ensure_identical() {
    file "$1"
    file "$2"
    diff "$1" "$2" # should produce no output
}

ensure_identical package1.deb package2.deb

# check that --debian, --rpm, --pacman work the same as --format and yield the deprecation warning

for PAIR in debian:deb rpm:rpm pacman:pkg.tar.xz; do
    FORMAT=${PAIR%:*}
    SUFFIX=${PAIR#*:}

    echo building with --${FORMAT} >&2
    ${HOLO_BUILD} -o package1.${SUFFIX} --${FORMAT}        ${INPUT_TOML}
    echo building with --format=${FORMAT} >&2
    ${HOLO_BUILD} -o package2.${SUFFIX} --format=${FORMAT} ${INPUT_TOML}

    ensure_identical package1.${SUFFIX} package2.${SUFFIX}
done

# check that --stdout and --no-stdout are deprecated, but work as advertised

echo building with --stdout >&2
${HOLO_BUILD} --format=debian --stdout   ${INPUT_TOML} > package1.deb
echo building with --output=- >&2
${HOLO_BUILD} --format=debian --output=- ${INPUT_TOML} > package2.deb

ensure_identical package1.deb package2.deb

echo building with --no-stdout >&2
${HOLO_BUILD} --format=debian --no-stdout ${INPUT_TOML}
mv package_1.0-1_all.deb package1.deb
echo building with --output=\"\" >&2
${HOLO_BUILD} --format=debian --output="" ${INPUT_TOML}
mv package_1.0-1_all.deb package2.deb

ensure_identical package1.deb package2.deb
