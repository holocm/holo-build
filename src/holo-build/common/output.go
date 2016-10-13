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
	"bytes"
	"io"
	"io/ioutil"
	"os"
)

//WriteOutput will write the generated package to a file (or stdout) if
//required.
func (pkg *Package) WriteOutput(generator Generator, pkgBytes []byte, printToStdout, withForce bool) (wasWritten bool, e error) {
	//print on stdout does not require additional logic
	if printToStdout {
		_, err := os.Stdout.Write(pkgBytes)
		return false, err
	}

	//only write file if content has changed
	pkgFile := generator.RecommendedFileName(pkg)
	if !withForce {
		fileHandle, err := os.Open(pkgFile)
		if err == nil {
			defer fileHandle.Close()
			equal, err := readerEqualTo(fileHandle, pkgBytes)
			if equal || err != nil {
				return false, err
			}
		} else {
			if !os.IsNotExist(err) {
				return false, err
			}
		}
	}

	return true, ioutil.WriteFile(pkgFile, pkgBytes, 0666)
}

//Return true if the reader contains exactly the given byte string.
func readerEqualTo(r io.Reader, str []byte) (bool, error) {
	buf := make([]byte, len(str))
	_, err := io.ReadFull(r, buf)
	switch err {
	case io.ErrUnexpectedEOF:
		return false, nil
	case nil:
		return bytes.Equal(buf, str), nil
	default:
		return false, err
	}
}
