/*******************************************************************************
*
* Copyright 2016 Stefan Majewsky <majewsky@gmx.net>
*
* This file is part of Holo.
*
* Holo is free software: you can redistribute it and/or modify it under the
* terms of the GNU General Public License as published by the Free Software
* Foundation, either version 3 of the License, or (at your option) any later
* version.
*
* Holo is distributed in the hope that it will be useful, but WITHOUT ANY
* WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR
* A PARTICULAR PURPOSE. See the GNU General Public License for more details.
*
* You should have received a copy of the GNU General Public License along with
* Holo. If not, see <http://www.gnu.org/licenses/>.
*
*******************************************************************************/

package main

import (
	"fmt"
	"os"
)

//ShowWarning prints a warning message on stderr.
func ShowWarning(msg string) {
	fmt.Fprintf(os.Stderr, "\x1b[33m\x1b[1m>>\x1b[0m %s\n", msg)
}

//WarnDeprecatedKey prints a warning message to inform the user that she has
//used a deprecated key in her package definition.
func WarnDeprecatedKey(key string) {
	ShowWarning("The '" + key + "' key is deprecated. See `man 1 holo-build` for details.")
}
