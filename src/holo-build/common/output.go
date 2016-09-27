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

package common

import (
	"io/ioutil"
	"os"
)

//WriteOutput will write the generated package to a file (or stdout) if
//required.
func (pkg *Package) WriteOutput(generator Generator, pkgBytes []byte, printToStdout bool) error {
	//print on stdout does not require additional logic
	if printToStdout {
		_, err := os.Stdout.Write(pkgBytes)
		return err
	}

	pkgFile := generator.RecommendedFileName(pkg)
	return ioutil.WriteFile(pkgFile, pkgBytes, 0666)
}
