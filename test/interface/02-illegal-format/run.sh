#!/bin/sh

# check that an unknown value for --format is rejected correctly

${HOLO_BUILD} --format=compact-disk ${INPUT_TOML}
