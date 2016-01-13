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

//#include <unistd.h>
import "C"
import "os"

/*******************************************************************************
 * FSNodeMetadata
 */

//IntOrString is used for FSNodeMetadata.Owner and FSNodeMetadata.Group that
//can be either int or string.
type IntOrString struct {
	Int uint32
	Str string
}

//FSNodeMetadata collects some metadata that is shared across FSNode-compatible
//types.
type FSNodeMetadata struct {
	Mode  os.FileMode
	Owner *IntOrString
	Group *IntOrString
}

//ApplyTo applies the metadata to the filesystem entry at the given path.
func (m *FSNodeMetadata) ApplyTo(path string) error {
	var uid C.__uid_t
	var gid C.__gid_t
	if m.Owner != nil {
		uid = C.__uid_t(m.Owner.Int)
	}
	if m.Group != nil {
		gid = C.__gid_t(m.Group.Int)
	}
	if uid != 0 || gid != 0 {
		//cannot use os.Chown(); os.Chown calls into syscall.Chown and thus
		//does a direct syscall which cannot be intercepted by fakeroot; I
		//need to call chown(2) via cgo
		result, err := C.chown(C.CString(path), uid, gid)
		if result != 0 && err != nil {
			return err
		}
	}
	return nil
}
