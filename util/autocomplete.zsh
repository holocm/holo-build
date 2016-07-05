#compdef holo-build

(( $+functions[_holo_build_zsh_comp] )) || _holo_build_zsh_comp()
{
    _arguments : \
        '--help[Print short usage information.]' \
        '--version[Print a short version string.]' \
        '(--stdout --no-stdout)--stdout[Print resulting package on stdout]' \
        '(--stdout --no-stdout)--no-stdout[Write resulting package to the working directory]' \
        '(--pacman --debian --rpm)--debian[Build a Debian package]' \
        '(--pacman --debian --rpm)--pacman[Build a pacman package]' \
        '(--pacman --debian --rpm)--rpm[Build an LSB-compliant RPM package (experimental!)]' \
        '::input file:_files'
    return 0
}

_holo_build_zsh_comp "$@"
