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

package pacman

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"strings"

	"../common"
)

//MakeMTREE generates the mtree metadata archive for this package.
func MakeMTREE(pkg *common.Package) ([]byte, error) {
	//this implementation is not particularly clever w.r.t. the use of "/set",
	//but we use some defaults here to maybe keep the result size down a bit
	lines := []string{
		"#mtree",
		"/set type=file uid=0 gid=0 mode=644 time=0.0",
	}

	pkg.WalkFSWithAbsolutePaths(func(path string, node common.FSNode) error {
		//skip root directory
		if path == "/" {
			return nil
		}

		//make path relative, e.g. "./etc/foo.conf"
		line := mtreeEscapeString("." + path)

		//add attributes in the same order as makepkg:
		//  type,uid,gid,mode,time,size,md5,sha256,link
		switch n := node.(type) {
		case *common.FSDirectory:
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
		case *common.FSRegularFile:
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
		case *common.FSSymlink:
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
