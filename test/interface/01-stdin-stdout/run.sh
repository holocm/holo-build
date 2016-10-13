#!/bin/sh

# test basic behavior: input from stdin, output to stdout

${HOLO_BUILD} --stdout --debian < ${INPUT_TOML} | file -
