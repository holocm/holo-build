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
	"errors"
	"io"
	"io/ioutil"
	"os"
)

//WriteOutput will write the generated package to a file (or stdout) if
//required. If the given file name is "-", stdout will be written to.
//If the given file name is empty, a name is chosen automatically.
func WriteOutput(pkgBytes []byte, pkgFile string, withForce bool) (wasWritten bool, e error) {
	//print on stdout does not require additional logic
	if pkgFile == "-" {
		_, err := os.Stdout.Write(pkgBytes)
		return false, err
	}

	//only write file if content has changed
	if !withForce {
		fileHandle, err := os.Open(pkgFile)
		if err == nil {
			defer fileHandle.Close()
			equal, err := readerEqualTo(fileHandle, pkgBytes)
			if equal || err != nil {
				return false, err
			}
			return true, errors.New("file already exists and has different contents; won't overwrite without --force")
		}

		if !os.IsNotExist(err) {
			return false, err
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
