#!/bin/bash
_holo_build() {
    COMPREPLY=( $(compgen -W "--help --version --stdout --no-stdout --reproducible --no-reproducible --pacman --debian" -- "${COMP_WORDS[COMP_CWORD]}") )
    return 0
}
complete -F _holo_build holo-build
