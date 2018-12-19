/*******************************************************************************
*
* Copyright 2015-2018 Stefan Majewsky <majewsky@gmx.net>
*
* Licensed under the Apache License, Version 2.0 (the "License");
* you may not use this file except in compliance with the License.
* You should have received a copy of the License along with this
* program. If not, you may obtain a copy of the License at
*
*     http://www.apache.org/licenses/LICENSE-2.0
*
* Unless required by applicable law or agreed to in writing, software
* distributed under the License is distributed on an "AS IS" BASIS,
* WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
* See the License for the specific language governing permissions and
* limitations under the License.
*
*******************************************************************************/

package pacman

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"strings"

	build "github.com/holocm/libpackagebuild"
	"github.com/holocm/libpackagebuild/filesystem"
)

//makeMTREE generates the mtree metadata archive for this package.
func makeMTREE(pkg *build.Package) ([]byte, error) {
	//this implementation is not particularly clever w.r.t. the use of "/set",
	//but we use some defaults here to maybe keep the result size down a bit
	lines := []string{
		"#mtree",
		"/set type=file uid=0 gid=0 mode=644 time=0.0",
	}

	pkg.WalkFSWithAbsolutePaths(func(path string, node filesystem.Node) error {
		//skip root directory
		if path == "/" {
			return nil
		}

		//make path relative, e.g. "./etc/foo.conf"
		line := mtreeEscapeString("." + path)

		//add attributes in the same order as makepkg:
		//  type,uid,gid,mode,time,size,md5,sha256,link
		switch n := node.(type) {
		case *filesystem.Directory:
			line += " type=dir"
			if uid := n.Metadata.UID(); uid != 0 { //uid 0 is default
				line += fmt.Sprintf(" uid=%d", uid)
			}
			if gid := n.Metadata.GID(); gid != 0 { //gid 0 is default
				line += fmt.Sprintf(" gid=%d", gid)
			}
			if n.Metadata.Mode != 0644 { //mode 0644 is default
				line += fmt.Sprintf(" mode=%o", n.Metadata.Mode)
			}
		case *filesystem.RegularFile:
			// type=file is default
			if uid := n.Metadata.UID(); uid != 0 { //uid 0 is default
				line += fmt.Sprintf(" uid=%d", uid)
			}
			if gid := n.Metadata.GID(); gid != 0 { //gid 0 is default
				line += fmt.Sprintf(" gid=%d", gid)
			}
			if n.Metadata.Mode != 0644 { //mode 0644 is default
				line += fmt.Sprintf(" mode=%o", n.Metadata.Mode)
			}
			line += fmt.Sprintf(" size=%d md5digest=%s sha256digest=%s",
				len([]byte(n.Content)), n.MD5Digest(), n.SHA256Digest(),
			)
		case *filesystem.Symlink:
			// uid=0 gid=0 is default
			line += " type=link mode=777"
			//need to replace spaces in link target since spaces separate
			line += " link=" + mtreeEscapeString(n.Target)
		}

		lines = append(lines, line)
		return nil
	})

	contents := strings.Join(lines, "\n") + "\n"

	//GZip that
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)

	_, err := w.Write([]byte(contents))
	if err != nil {
		return nil, err
	}

	err = w.Close()
	return buf.Bytes(), err
}

//From the mtree(5) manpage:
//
//> When encoding file or pathnames, any backslash character or character
//> outside of the 95 printable ASCII characters must be encoded as a a
//> backslash followed by three octal digits.
func mtreeEscapeString(input string) string {
	//this conversion takes place on the byte level
	in := []byte(input)
	out := make([]byte, 0, len(in))

	for _, byt := range in {
		if byt > ' ' && byt <= '~' {
			//pass printable non-whitespace ASCII characters through directly
			out = append(out, byt)
		} else {
			//write escape sequence for non-printable, non-ASCII characters or space
			out = append(out, fmt.Sprintf("\\%03o", byt)...)
		}
	}

	return string(out)
}
