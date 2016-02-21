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
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"sort"
)

//FSNode instances represent an entry in the file system (such as a file or a
//directory).
type FSNode interface {
	//Insert inserts a new node below the current node at the given relative
	//path. The path is given as a slice of strings, separated on slashes, e.g.
	//`[]string{"var","lib","foo"}` for the path `"var/lib/foo".
	//
	//The `location` argument contains the absolute path to the current node;
	//this can be used for error reporting.
	Insert(entry FSNode, relPath []string, location string) error
	//InstalledSizeInBytes approximates the apparent size of the given
	//directory and everything in it, as calculated by `du -s --apparent-size`,
	//but in a filesystem-independent way.
	InstalledSizeInBytes() int
	//FileModeForArchive returns the file mode of this FSNode as stored in a
	//tar or CPIO archive.
	FileModeForArchive(includingFileType bool) uint32
	//Walk visits all the nodes below this FSNode (including itself) and calls
	//the given callback at each node. It is guaranteed that the callback for a
	//node is called after the callback of its parent node (if any).
	Walk(absolutePath string, callback func(absolutePath string, node FSNode) error) error
}

////////////////////////////////////////////////////////////////////////////////
// FSNodeMetadata
//

//IntOrString is used for FSNodeMetadata.Owner and FSNodeMetadata.Group that
//can be either int or string.
//
//Note that, from within a generator, you will always see `Str` to be empty.
//See PostponeUnmaterializable() for details.
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

//UID returns Owner.Int if it is set.
func (m *FSNodeMetadata) UID() uint32 {
	if m.Owner != nil {
		return m.Owner.Int
	}
	return 0
}

//GID returns Group.Int if it is set.
func (m *FSNodeMetadata) GID() uint32 {
	if m.Group != nil {
		return m.Group.Int
	}
	return 0
}

//PostponeUnmaterializable generates an addition to the package's setup script
//to handle metadata at install-time that cannot be materialized at build-time
//(namely owners/groups identified by name which cannot be resolved into
//numeric IDs at build time).
func (m *FSNodeMetadata) PostponeUnmaterializable(path string) (additionalSetupScript string) {
	var ownerStr, groupStr string
	if m.Owner != nil && m.Owner.Str != "" {
		ownerStr = m.Owner.Str
		m.Owner = nil
	}
	if m.Group != nil && m.Group.Str != "" {
		groupStr = m.Group.Str
		m.Group = nil
	}

	if ownerStr != "" {
		if groupStr != "" {
			return fmt.Sprintf("chown %s:%s %s\n", ownerStr, groupStr, path)
		}
		return fmt.Sprintf("chown %s %s\n", ownerStr, path)
	}
	if groupStr != "" {
		return fmt.Sprintf("chgrp %s %s\n", groupStr, path)
	}
	return ""
}

////////////////////////////////////////////////////////////////////////////////
// FSDirectory
//

//FSDirectory is a type of FSNode that represents directories. This FSNode
//references the nodes contained in the directory recursively.
type FSDirectory struct {
	Entries  map[string]FSNode
	Metadata FSNodeMetadata
	Implicit bool
}

//NewFSDirectory initializes an empty FSDirectory.
func NewFSDirectory() *FSDirectory {
	return &FSDirectory{
		Entries:  make(map[string]FSNode),
		Metadata: FSNodeMetadata{Mode: 0755},
	}
}

//Insert implements the FSNode interface.
func (d *FSDirectory) Insert(entry FSNode, relPath []string, location string) error {
	if len(relPath) == 0 {
		return errors.New("duplicate entry")
	}

	subname := relPath[0]
	subentry := d.Entries[subname]

	if len(relPath) == 1 {
		//entry is directly below this directory -> try to insert it
		if subentry != nil {
			//there is already an entry at this location -- if it's a directory
			//that was explicitly spawned, replace it by the explicitly
			//constructed entry silently; otherwise the entry is a duplicate
			dirOld, ok1 := subentry.(*FSDirectory)
			dirNew, ok2 := entry.(*FSDirectory)
			if !(ok1 && ok2 && dirOld.Implicit) {
				return errors.New("duplicate entry")
			}
			//don't lose the entries below the implicitly created directory
			for key, value := range dirOld.Entries {
				dirNew.Entries[key] = value
			}
		}
		d.Entries[subname] = entry
		return nil
	}

	//entry is inside a subdirectory of this one -> spawn the next child if
	//necessary and recurse
	if subentry == nil {
		subentry = NewFSDirectory()
		subentry.(*FSDirectory).Implicit = true //this node was implicitly created (see above)
		d.Entries[subname] = subentry
	}
	subdir, ok := subentry.(*FSDirectory)
	if !ok {
		return fmt.Errorf("%s/%s is not a directory", location, relPath[0])
	}
	return subdir.Insert(entry, relPath[1:], location+"/"+subname)
}

//InstalledSizeInBytes implements the FSNode interface.
func (d *FSDirectory) InstalledSizeInBytes() int {
	//sum over all entries
	sum := 0
	for _, entry := range d.Entries {
		sum += entry.InstalledSizeInBytes()
	}
	//contribution from the directory itself
	return sum + 4096
}

//FileModeForArchive implements the FSNode interface.
func (d *FSDirectory) FileModeForArchive(includingFileType bool) uint32 {
	if includingFileType {
		return 040000 | (uint32(d.Metadata.Mode) & 07777)
	}
	return uint32(d.Metadata.Mode) & 07777
}

//Walk implements the FSNode interface.
func (d *FSDirectory) Walk(absolutePath string, callback func(string, FSNode) error) error {
	err := callback(absolutePath, d)
	if err != nil {
		return err
	}

	//walk through entries in reproducible, sorted order
	names := make([]string, 0, len(d.Entries))
	for name := range d.Entries {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		entry := d.Entries[name]
		var nextPath string
		switch absolutePath {
		case "":
			nextPath = name
		case "/":
			nextPath = "/" + name
		default:
			nextPath = absolutePath + "/" + name
		}
		err = entry.Walk(nextPath, callback)
		if err != nil {
			return err
		}
	}
	return nil
}

////////////////////////////////////////////////////////////////////////////////
// FSRegularFile
//

//FSRegularFile is a type of FSNode that represents regular files.
type FSRegularFile struct {
	Content  string
	Metadata FSNodeMetadata
}

//Insert implements the FSNode interface.
func (f *FSRegularFile) Insert(entry FSNode, relPath []string, location string) error {
	if len(relPath) == 0 {
		return errors.New("duplicate entry")
	}
	return fmt.Errorf("%f is not a directory", location)
}

//InstalledSizeInBytes implements the FSNode interface.
func (f *FSRegularFile) InstalledSizeInBytes() int {
	return len(f.Content)
}

//FileModeForArchive implements the FSNode interface.
func (f *FSRegularFile) FileModeForArchive(includingFileType bool) uint32 {
	if includingFileType {
		return 0100000 | (uint32(f.Metadata.Mode) & 07777)
	}
	return uint32(f.Metadata.Mode) & 07777
}

//Walk implements the FSNode interface.
func (f *FSRegularFile) Walk(absolutePath string, callback func(string, FSNode) error) error {
	return callback(absolutePath, f)
}

//MD5Digest returns the MD5 digest of this file's contents.
func (f *FSRegularFile) MD5Digest() string {
	//the following is equivalent to sum := md5.Sum([]byte(f.Content)),
	//but also is backwards-compatible to Go 1.1
	digest := md5.New()
	digest.Write([]byte(f.Content))
	sum := digest.Sum(nil)

	return hex.EncodeToString(sum[:])
}

//SHA256Digest returns the SHA256 digest of this file's contents.
func (f *FSRegularFile) SHA256Digest() string {
	//the following is equivalent to sum := sha256.Sum([]byte(f.Content)),
	//but also is backwards-compatible to Go 1.1
	digest := sha256.New()
	digest.Write([]byte(f.Content))
	sum := digest.Sum(nil)

	return hex.EncodeToString(sum[:])
}

////////////////////////////////////////////////////////////////////////////////
// FSSymlink
//

//FSSymlink is a type of FSNode that represents symbolic links,
type FSSymlink struct {
	Target string
}

//Insert implements the FSNode interface.
func (s *FSSymlink) Insert(entry FSNode, relPath []string, location string) error {
	if len(relPath) == 0 {
		return errors.New("duplicate entry")
	}
	return fmt.Errorf("%f is not a directory", location)
}

//InstalledSizeInBytes implements the FSNode interface.
func (s *FSSymlink) InstalledSizeInBytes() int {
	return len(s.Target)
}

//FileModeForArchive implements the FSNode interface.
func (s *FSSymlink) FileModeForArchive(includingFileType bool) uint32 {
	if includingFileType {
		return 0120777
	}
	return 0777
}

//Walk implements the FSNode interface.
func (s *FSSymlink) Walk(absolutePath string, callback func(string, FSNode) error) error {
	return callback(absolutePath, s)
}
