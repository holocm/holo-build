#!/bin/sh
#
# Copyright 2015 Stefan Majewsky <majewsky@gmx.net>
#
# This file is part of Holo.
#
# Holo is free software: you can redistribute it and/or modify it under the
# terms of the GNU General Public License as published by the Free Software
# Foundation, either version 3 of the License, or (at your option) any later
# version.
#
# Holo is distributed in the hope that it will be useful, but WITHOUT ANY
# WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR
# A PARTICULAR PURPOSE. See the GNU General Public License for more details.
#
# You should have received a copy of the GNU General Public License along with
# Holo. If not, see <http://www.gnu.org/licenses/>.
#

# if a package format was specified explicitly, skip distribution detection
# (can also shortcut if just asked for --help or --version)
for ARG in "$@"; do
    case $ARG in
        --format|--debian|--pacman|--rpm|--help|--version)
            exec /usr/lib/holo/holo-build "$@" ;;
        *) ;;
    esac
done

# check distribution and choose appropriate package format
[ -f /etc/os-release ] && source /etc/os-release || source /usr/lib/os-release
DIST_IDS="$(echo "$ID $ID_LIKE" | tr ' ' ',')"

case ",$DIST_IDS," in
    *,arch,*)   exec /usr/lib/holo/holo-build --format=pacman "$@" ;;
    *,debian,*) exec /usr/lib/holo/holo-build --format=debian "$@" ;;
    *,fedora,*) exec /usr/lib/holo/holo-build --format=rpm "$@" ;;
    *,mageia,*) exec /usr/lib/holo/holo-build --format=rpm "$@" ;;
    *,suse,*)   exec /usr/lib/holo/holo-build --format=rpm "$@" ;;
    *)
        echo "!! Running on an unrecognized distribution. Distribution IDs: $DIST_IDS" >&2
        echo ">> Please report this error at <https://github.com/holocm/holo-build/issues/new>" >&2
        echo ">> and include the contents of your /etc/os-release file." >&2
        ;;
esac
