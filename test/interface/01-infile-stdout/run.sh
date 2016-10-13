#!/bin/sh

# test basic behavior: input from file, output to stdout

${HOLO_BUILD} --stdout --debian ${INPUT_TOML} | file -
