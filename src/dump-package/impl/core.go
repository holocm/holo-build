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

package impl

import (
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
)

//Indent is a general-purpose helper function for pretty-printing of nested data.
func Indent(dump string) string {
	//indent the first line and all subsequent lines except for the trailing newline
	//(and also ensure a trailing newline, which means that in total we can
	//trim the trailing newline at the start, and put it back at the end)
	dump = strings.TrimSuffix(dump, "\n")
	indent := "    "
	dump = indent + strings.Replace(dump, "\n", "\n"+indent, -1)
	return dump + "\n"
}

//RecognizeAndDump converts binary input data into a readable dump (if it can
//recognize the data format).
func RecognizeAndDump(data []byte) (string, error) {
	if len(data) == 0 {
		return "empty file\n", nil
	}

	//Thanks to https://stackoverflow.com/a/19127748/334761 for
	//listing all the magic numbers of the usual compression formats.

	//is it GZip-compressed?
	if bytes.HasPrefix(data, []byte{0x1f, 0x8b, 0x08}) {
		return dumpGZ(data)
	}
	//is it BZip2-compressed?
	if bytes.HasPrefix(data, []byte{0x42, 0x5a, 0x68}) {
		return dumpBZ2(data)
	}
	//is it XZ-compressed?
	if bytes.HasPrefix(data, []byte{0xfd, 0x37, 0x7a, 0x58, 0x5a, 0x00}) {
		return dumpXZ(data)
	}
	//is it LZMA-compressed?
	if bytes.HasPrefix(data, []byte{0x5d, 0x00, 0x00}) {
		return dumpLZMA(data)
	}
	//is it a POSIX tar archive?
	if len(data) >= 512 && bytes.Equal(data[257:262], []byte("ustar")) {
		return DumpTar(data)
	}
	//is it an mtree archive?
	if bytes.HasPrefix(data, []byte("#mtree")) {
		return DumpMtree(data)
	}
	//is it an ar archive?
	if bytes.HasPrefix(data, []byte("!<arch>\n")) {
		return DumpAr(data)
	}
	//is it a cpio archive?
	if bytes.HasPrefix(data, []byte("070701")) {
		return DumpCpio(data)
	}
	//is it an RPM package?
	if bytes.HasPrefix(data, []byte{0xed, 0xab, 0xee, 0xdb}) {
		return DumpRpm(data)
	}

	return "data as shown below\n" + Indent(string(data)), nil
}

func dumpGZ(data []byte) (string, error) {
	//use "compress/gzip" package to decompress the data
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return "", err
	}

	data2, err := ioutil.ReadAll(r)
	if err != nil {
		return "", err
	}

	//`data2` now contains the decompressed data
	dump, err := RecognizeAndDump(data2)
	return "GZip-compressed " + dump, err
}

func dumpBZ2(data []byte) (string, error) {
	//use "compress/bzip2" package to decompress the data
	r := bzip2.NewReader(bytes.NewReader(data))
	data2, err := ioutil.ReadAll(r)
	if err != nil {
		return "", err
	}

	//`data2` now contains the decompressed data
	dump, err := RecognizeAndDump(data2)
	return "BZip2-compressed " + dump, err
}

func dumpXZ(data []byte) (string, error) {
	return dumpUsingProgram(data, "XZ", "xz", "-d")
}

func dumpLZMA(data []byte) (string, error) {
	return dumpUsingProgram(data, "LZMA", "xz", "--format=lzma", "--decompress", "--stdout")
}

func dumpUsingProgram(data []byte, format string, command string, args ...string) (string, error) {
	cmd := exec.Command(command, args...)
	cmd.Stdin = bytes.NewReader(data)
	cmd.Stderr = os.Stderr
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	//`output` now contains the decompressed data
	dump, err := RecognizeAndDump(output)
	return format + "-compressed " + dump, err
}
