#!/bin/sh

# test basic behavior: input from file, output to stdout

${HOLO_BUILD} -o - --format=debian ${INPUT_TOML} | file -
