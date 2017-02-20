#!/bin/bash
_holo_build() {
    local cur="${COMP_WORDS[COMP_CWORD]}"
    local prev="${COMP_WORDS[COMP_CWORD-1]}"
    if [[ $cur = -* ]]; then
        COMPREPLY=( $(compgen -W "-f --force --format --help -o --output --suggest-filename -V --version" -- "$cur") )
    elif [ "$COMP_CWORD" -gt 0 ]; then
        if [[ $prev = --format ]]; then
            COMPREPLY=( $(compgen -W "debian pacman rpm" -- "$cur") )
        fi
    fi
}
complete -o default -F _holo_build holo-build
