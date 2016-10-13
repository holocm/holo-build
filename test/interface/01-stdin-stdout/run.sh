#!/bin/sh

# test basic behavior: input from stdin, output to stdout

${HOLO_BUILD} -o - --format=debian < ${INPUT_TOML} | file -
