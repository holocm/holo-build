/*******************************************************************************
*
* Copyright 2015 Stefan Majewsky <majewsky@gmx.net>
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

//#include <fcntl.h>
//#include <sys/stat.h>
import "C"
import (
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

//ResetTimestamp sets the atime and mtime of the given file to 0 (i.e.
//1970-01-01T00:00:00Z). This is required only when building a --reproducible
//package.
func ResetTimestamp(path string) error {
	timespecs := []syscall.Timespec{
		syscall.Timespec{Sec: 0, Nsec: 0},
		syscall.Timespec{Sec: 0, Nsec: 0},
	}

	//we cannot use os.Chtimes() here because we need AT_SYMLINK_NOFOLLOW to
	//set the times of a symlink itself rather than its target
	result, err := C.utimensat(
		C.AT_FDCWD, C.CString(path),
		(*C.struct_timespec)(unsafe.Pointer(&timespecs[0])), // urgh
		C.AT_SYMLINK_NOFOLLOW,
	)
	if result == 0 {
		return nil
	}
	return err
}

//WriteFile acts like ioutil.WriteFile, but calls ResetTimestamp if buildReproducibly is true.
func WriteFile(path string, contents []byte, mode os.FileMode, buildReproducibly bool) error {
	err := ioutil.WriteFile(path, contents, mode)
	if err != nil {
		return err
	}
	if buildReproducibly {
		return ResetTimestamp(path)
	}
	return nil
}

//InstalledSizeInBytes approximates the apparent size of the given directory
//and everything in it, as calculated by `du -s --apparent-size`, but in a
//filesystem-independent way.
func (pkg *Package) InstalledSizeInBytes() int {
	//count all directories in the package as contributing 4096 bytes, and all
	//files and symlinks as contributing their content
	contributions := make(map[string]int, len(pkg.FSEntries))

	//the root directory is always there
	contributions["/"] = 4096

	for _, entry := range pkg.FSEntries {
		//contributions for this entry
		path := entry.Path
		if entry.Type == FSEntryTypeDirectory {
			contributions[path] = 4096
		} else {
			contributions[path] = len(entry.Content)
		}

		//contributions for all directories above it
		for path != "/" {
			path = filepath.Dir(path)
			contributions[path] = 4096
		}
	}

	//sum over all contributions
	size := 0
	for _, contribution := range contributions {
		size += contribution
	}
	return size
}
