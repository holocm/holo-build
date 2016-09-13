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
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sort"
	"strings"

	"../../../localdeps/github.com/blakesmith/ar"
	cpio "../../../localdeps/github.com/surma/gocpio"
)

//DumpTar dumps tar archives.
func DumpTar(data []byte, withChecksums bool) (string, error) {
	//use "archive/tar" package to read the tar archive
	tr := tar.NewReader(bytes.NewReader(data))
	var header *tar.Header
	var err error

	return dumpArchiveGeneric(
		"POSIX tar archive", withChecksums, tr,
		func() (string, error) { //func gotoNextEntry
			header, err = tr.Next()
			if err != nil {
				return "", err
			}
			return header.Name, nil
		},
		func(idx int) (string, bool, bool, error) { //func describeEntry
			info := header.FileInfo()

			//recognize entry type
			str := ""
			isRegular := false
			switch info.Mode() & os.ModeType {
			case os.ModeDir:
				str = "directory"
			case os.ModeSymlink:
				str = "symlink"
			case 0:
				str = "regular file"
				isRegular = true
			default:
				return "", false, false, fmt.Errorf("tar entry %s has unrecognized file mode (%o)", header.Name, info.Mode())
			}

			//add metadata
			if str == "symlink" {
				str += fmt.Sprintf(" to %s", header.Linkname)
			} else {
				str += fmt.Sprintf(" (mode: %o, owner: %d, group: %d)",
					info.Mode()&os.ModePerm, header.Uid, header.Gid,
				)
			}

			return str, isRegular, false, nil
		},
	)
}

//DumpAr dumps ar archives.
func DumpAr(data []byte, withChecksums bool) (string, error) {
	var header *ar.Header
	var err error
	//use "github.com/blakesmith/ar" package to read the ar archive
	ar := ar.NewReader(bytes.NewReader(data))

	return dumpArchiveGeneric(
		"ar archive", withChecksums, ar,
		func() (string, error) { //func gotoNextEntry
			header, err = ar.Next()
			if err != nil {
				return "", err
			}
			return header.Name, nil
		},
		func(idx int) (string, bool, bool, error) { //func describeEntry
			//our ar parser only works with a small subset of all the varieties of
			//ar files (large enough to handle Debian packages whose toplevel ar
			//packages contain just plain files with short names), so we assume
			//that everything that it reads without crashing is a regular file
			str := fmt.Sprintf("regular file (mode: %o, owner: %d, group: %d)",
				header.Mode, header.Uid, header.Gid,
			)

			//for Debian packages, we need to check that the file "debian-binary"
			//is the first entry
			if header.Name == "debian-binary" {
				str += fmt.Sprintf(" at archive position %d", idx)
			}

			return str, true, false, nil
		},
	)
}

//DumpCpio dumps cpio archives.
func DumpCpio(data []byte, withChecksums bool) (string, error) {
	//use "github.com/surma/gocpio" package to read the ar archive
	cr := cpio.NewReader(bytes.NewReader(data))
	var header *cpio.Header
	var err error

	return dumpArchiveGeneric(
		"cpio archive", withChecksums, cr,
		func() (string, error) { //func gotoNextEntry
			header, err = cr.Next()
			if err != nil {
				return "", err
			}
			if header.IsTrailer() {
				return "", io.EOF
			}
			return header.Name, nil
		},
		func(idx int) (string, bool, bool, error) { //func describeEntry
			//recognize entry type
			str := ""
			isRegular, isSymlink := false, false
			switch header.Type {
			case cpio.TYPE_SOCK:
				str = "socket"
			case cpio.TYPE_SYMLINK:
				str = "symlink"
				isSymlink = true
			case cpio.TYPE_REG:
				str = "regular file"
				isRegular = true
			case cpio.TYPE_BLK:
				str = "block special devices"
			case cpio.TYPE_DIR:
				str = "directory"
			case cpio.TYPE_CHAR:
				str = "character special device"
			case cpio.TYPE_FIFO:
				str = "named pipe (FIFO)"
			}

			//add metadata
			if !isSymlink {
				str += fmt.Sprintf(" (mode: %o, owner: %d, group: %d)",
					header.Mode, header.Uid, header.Gid,
				)
			}

			return str, isRegular, isSymlink, nil
		},
	)
}

//The generic parts of DumpTar, DumpAr and DumpCpio.
func dumpArchiveGeneric(typeString string, withChecksums bool, reader io.Reader, gotoNextEntry func() (string, error), describeEntry func(idx int) (string, bool, bool, error)) (string, error) {
	dumps := make(map[string]string)
	var names []string

	//iterate through the entries in the archive
	idx := -1
	for {
		idx++

		//get next entry
		name, err := gotoNextEntry()
		if err == io.EOF {
			break //end of archive
		}
		if err != nil {
			return "", err
		}

		//get contents of entry
		data, err := ioutil.ReadAll(reader)
		if err != nil {
			return "", err
		}

		//get entry description (containing a serialization of metadata)
		description, isRegular, isSymlink, err := describeEntry(idx)
		str := fmt.Sprintf(">> %s is %s", name, description)

		//for regular files, include a dump of the contents
		if isRegular {
			dump, err := RecognizeAndDump(data, withChecksums)
			if err != nil {
				return "", err
			}
			str += ", content is " + dump
		} else {
			//if isSymlink is set, `data` contains the symlink target
			if isSymlink {
				str += " to " + string(data)
			}
			str += "\n"
		}

		names = append(names, name)
		dumps[name] = str
	}

	//dump entries ordered by name
	sort.Strings(names)
	dump := ""
	for _, name := range names {
		dump += dumps[name]
	}

	return typeString + "\n" + Indent(dump), nil
}

//DumpMtree dumps mtree metadata archives.
func DumpMtree(data []byte) (string, error) {
	//We don't have a library for the mtree(5) format, but it's relatively simple.
	//NOTE: We don't support absolute paths ("mtree v2.0") and we don't track the cwd.
	//All we do is resolve duplicate entries and "/set" and "/unset" commands.
	lines := strings.Split(string(data), "\n")

	//go through each entry and resolve "/set"
	globalOpts := make(map[string]string)
	entries := make(map[string]map[string]string)

	for _, line := range lines {
		//ignore comments
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}

		//lines look like "name option option option"
		options := strings.Split(line, " ")
		name := options[0]
		options = options[1:]

		//parse options (option = "key=value")
		opts := make(map[string]string, len(options))
		for _, option := range options {
			pair := strings.SplitN(option, "=", 2)
			if len(pair) == 1 {
				opts[pair[0]] = ""
			} else {
				opts[pair[0]] = pair[1]
			}
		}

		//name can either be a special command or a filename
		switch name {
		case "/set":
			//set the opts globally
			for key, value := range opts {
				globalOpts[key] = value
			}
		case "/unset":
			//unset the opts globally
			for key := range opts {
				delete(globalOpts, key)
			}
		default:
			//create (if missing) an entry for this file and add the opts to it
			entry, ok := entries[name]
			if !ok {
				entry = make(map[string]string, len(opts)+len(globalOpts))
				//apply globalOpts
				for key, value := range globalOpts {
					entry[key] = value
				}
				entries[name] = entry
			}
			for key, value := range opts {
				entry[key] = value
			}
		}
	}

	//sort entries by name
	entryNames := make([]string, 0, len(entries))
	for name := range entries {
		entryNames = append(entryNames, name)
	}
	sort.Strings(entryNames)

	outputLines := make([]string, 0, len(entries))
	for _, name := range entryNames {
		//sort options for entry by key
		entry := entries[name]
		keys := make([]string, 0, len(entry))
		for key := range entry {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		options := ""
		for _, key := range keys {
			options += fmt.Sprintf(" %s=%s", key, entry[key])
		}

		outputLines = append(outputLines, ">> "+name+options)
	}

	return "mtree metadata archive\n" + Indent(strings.Join(outputLines, "\n")), nil
}
