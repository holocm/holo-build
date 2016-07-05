#!/bin/bash
_holo_build() {
    local cur="${COMP_WORDS[COMP_CWORD]}"
    if [[ $cur = -* ]]; then
        COMPREPLY=( $(compgen -W "--help --version --stdout --no-stdout --pacman --debian --rpm" -- "$cur") )
    fi
}
complete -o default -F _holo_build holo-build
