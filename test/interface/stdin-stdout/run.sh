#!/bin/sh
${HOLO_BUILD} --stdout --debian < ${INPUT_TOML} | file -
