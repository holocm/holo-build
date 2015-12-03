#compdef holo-build

(( $+functions[_holo_build_zsh_comp] )) || _holo_build_zsh_comp()
{
    _arguments : \
        '--help[Print short usage information.]' \
        '--version[Print a short version string.]' \
        '(--stdout --no-stdout)--stdout[Print resulting package on stdout]' \
        '(--stdout --no-stdout)--no-stdout[Write resulting package to the working directory]' \
        '(--reproducible --no-reproducible)--reproducible[Build a reproducible package with bogus timestamps etc.]' \
        '(--reproducible --no-reproducible)--no-reproducible[Build a non-reproducible package with actual timestamps etc.]' \
        '(--pacman --debian)--debian[Build a Debian package]'
        '(--pacman --debian)--pacman[Build a pacman package]'
    return 0
}

_holo_build_zsh_comp "$@"
