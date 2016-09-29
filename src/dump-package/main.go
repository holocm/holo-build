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

package main

// #include <locale.h>
import "C"
import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/holocm/holo-build/src/dump-package/impl"
)

//This program is used by the holo-build tests to extract generated packages and render
//a textual representation of the package, including the compression and
//archive formats used and all file metadata contained within the archives.
//The program is called like
//
//    ./build/dump-package < $package
//
//And renders output like this:
//
//    $ tar cJf foo.tar.xz foo/
//    $ ./build/dump-package < foo.tar.xz
//    XZ-compressed data
//        POSIX tar archive
//            >> foo/ is directory (mode: 0755, owner: 1000, group: 1000)
//            >> foo/bar is regular file (mode: 0600, owner: 1000, group: 1000), content: data as shown below
//                Hello World!
//            >> foo/baz is symlink to bar
//
//The program is deliberately written very generically so as to make it easy to
//add support for new package formats in the future (when holo-build gains new
//generators).

func main() {
	//Holo requires a neutral locale, esp. for deterministic sorting of file paths
	lcAll := C.int(0)
	C.setlocale(lcAll, C.CString("C"))

	//check arguments
	withChecksums := len(os.Args) > 1 && os.Args[1] == "--with-checksums"

	//read the input from stdin
	data, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	//recognize the input, while deconstructing it recursively
	dump, err := impl.RecognizeAndDump(data, withChecksums)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	fmt.Println(dump)
}
