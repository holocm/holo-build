#!/bin/sh

# test basic behavior: input from file, output to file

${HOLO_BUILD} --debian ${INPUT_TOML}
file package_1.0-1_all.deb