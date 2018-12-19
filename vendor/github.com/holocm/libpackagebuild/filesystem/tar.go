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

package filesystem

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

//ToTarArchive creates a TAR archive containing this directory and all the
//filesystem entries in it.
//
//With `leadingDot = true`, generate entry paths like `./foo/bar.conf`.
//With `leadingDot = false`, generate entry paths like `foo/bar.conf`.
//
//With `skipRootDirectory = true`, don't generate an entry for the root
//directory in the resulting package.
func (d *Directory) ToTarArchive(w io.Writer, leadingDot, skipRootDirectory bool) error {
	tw := tar.NewWriter(w)

	timestamp := time.Unix(0, 0)

	err := d.Walk(".", func(path string, node Node) error {
		if !leadingDot {
			path = strings.TrimPrefix(path, "./")
		}
		if skipRootDirectory && path == "." {
			return nil
		}

		var err error
		switch n := node.(type) {
		case *Directory:
			err = tw.WriteHeader(&tar.Header{
				Name:       path + "/",
				Typeflag:   tar.TypeDir,
				Mode:       int64(n.FileModeForArchive(false)),
				Uid:        int(n.Metadata.UID()),
				Gid:        int(n.Metadata.GID()),
				ModTime:    timestamp,
				AccessTime: timestamp,
				ChangeTime: timestamp,
			})
		case *RegularFile:
			err = tw.WriteHeader(&tar.Header{
				Name:       path,
				Size:       int64(len([]byte(n.Content))),
				Typeflag:   tar.TypeReg,
				Mode:       int64(n.FileModeForArchive(false)),
				Uid:        int(n.Metadata.UID()),
				Gid:        int(n.Metadata.GID()),
				ModTime:    timestamp,
				AccessTime: timestamp,
				ChangeTime: timestamp,
			})
		case *Symlink:
			err = tw.WriteHeader(&tar.Header{
				Name:       path,
				Typeflag:   tar.TypeSymlink,
				Mode:       int64(n.FileModeForArchive(false)),
				Linkname:   n.Target,
				ModTime:    timestamp,
				AccessTime: timestamp,
				ChangeTime: timestamp,
			})
		default:
			panic("unreachable")
		}
		if err != nil {
			return err
		}
		if n, ok := node.(*RegularFile); ok {
			_, err = tw.Write([]byte(n.Content))
			return err
		}
		return nil
	})
	if err != nil {
		tw.Close()
		return err
	}

	return tw.Close()
}

//ToTarGZArchive is identical to ToTarArchive, but GZip-compresses the result.
func (d *Directory) ToTarGZArchive(w io.Writer, leadingDot, skipRootDirectory bool) error {
	gzw := gzip.NewWriter(w)

	err := d.ToTarArchive(gzw, leadingDot, skipRootDirectory)
	if err != nil {
		gzw.Close()
		return err
	}
	return gzw.Close()
}

//ToTarXZArchive is identical to ToTarArchive, but GZip-compresses the result.
func (d *Directory) ToTarXZArchive(w io.Writer, leadingDot, skipRootDirectory bool) error {
	var buf bytes.Buffer
	err := d.ToTarArchive(&buf, leadingDot, skipRootDirectory)
	if err != nil {
		return err
	}

	//since we don't have a "compress/xz" package, use the "xz" binary instead
	cmd := exec.Command("xz", "--compress")
	cmd.Stdin = &buf
	cmd.Stdout = w
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
