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

//Package filesystem contains a simple representation of hierarchical
//filesystems that is used by libpackagebuild.
package filesystem

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

//Node instances represent an entry in the file system (such as a file or a
//directory).
type Node interface {
	//Insert inserts a new node below the current node at the given relative
	//path. The path is given as a slice of strings, separated on slashes, e.g.
	//`[]string{"var","lib","foo"}` for the path `"var/lib/foo"`.
	//
	//The `location` argument contains the absolute path to the current node;
	//this can be used for error reporting.
	Insert(entry Node, relPath []string, location string) error
	//InstalledSizeInBytes approximates the apparent size of the given
	//directory and everything in it, as calculated by `du -s --apparent-size`,
	//but in a filesystem-independent way.
	InstalledSizeInBytes() int
	//FileModeForArchive returns the file mode of this Node as stored in a
	//tar or CPIO archive.
	FileModeForArchive(includingFileType bool) uint32
	//Walk visits all the nodes below this Node (including itself) and calls
	//the given callback at each node. It is guaranteed that the callback for a
	//node is called after the callback of its parent node (if any).
	//
	//The `callback` can arrange to skip over a directory by returning
	//filepath.SkipDir.
	Walk(absolutePath string, callback func(absolutePath string, node Node) error) error
	//PostponeUnmaterializable generates a shell script that applies all
	//filesystem metadata in this node and its children that cannot be
	//represented in a tar archive directly. Specifically, owners/groups
	//identified by name cannot be resolved into numeric IDs at build time, so
	//this call will generate a shell script calling chown/chmod/chgrp as
	//required.
	PostponeUnmaterializable(absolutePath string) string
}

////////////////////////////////////////////////////////////////////////////////
// NodeMetadata
//

//IntOrString is used for NodeMetadata.Owner and NodeMetadata.Group that
//can be either int or string.
//
//Note that, from within a generator, you will always see `Str` to be empty.
//See Node.PostponeUnmaterializable() for details.
type IntOrString struct {
	Int uint32
	Str string
}

//NodeMetadata collects some metadata that is shared across Node-compatible
//types.
type NodeMetadata struct {
	Mode  os.FileMode
	Owner *IntOrString
	Group *IntOrString
}

//UID returns Owner.Int if it is set.
func (m *NodeMetadata) UID() uint32 {
	if m.Owner != nil {
		return m.Owner.Int
	}
	return 0
}

//GID returns Group.Int if it is set.
func (m *NodeMetadata) GID() uint32 {
	if m.Group != nil {
		return m.Group.Int
	}
	return 0
}

//PostponeUnmaterializable generates an addition to the package's setup script
//to handle metadata at install-time that cannot be materialized at build-time
//(namely owners/groups identified by name which cannot be resolved into
//numeric IDs at build time).
func (m *NodeMetadata) postponeUnmaterializable(path string) (additionalSetupScript string) {
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
// Directory
//

//Directory is a type of Node that represents directories. This Node
//references the nodes contained in the directory recursively.
type Directory struct {
	Entries  map[string]Node
	Metadata NodeMetadata
	Implicit bool
}

//NewDirectory initializes an empty Directory.
func NewDirectory() *Directory {
	return &Directory{
		Entries:  make(map[string]Node),
		Metadata: NodeMetadata{Mode: 0755},
	}
}

//Insert implements the Node interface.
func (d *Directory) Insert(entry Node, relPath []string, location string) error {
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
			dirOld, ok1 := subentry.(*Directory)
			dirNew, ok2 := entry.(*Directory)
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
		subentry = NewDirectory()
		subentry.(*Directory).Implicit = true //this node was implicitly created (see above)
		d.Entries[subname] = subentry
	}
	subdir, ok := subentry.(*Directory)
	if !ok {
		return fmt.Errorf("%s/%s is not a directory", location, relPath[0])
	}
	return subdir.Insert(entry, relPath[1:], location+"/"+subname)
}

//InstalledSizeInBytes implements the Node interface.
func (d *Directory) InstalledSizeInBytes() int {
	//sum over all entries
	sum := 0
	for _, entry := range d.Entries {
		sum += entry.InstalledSizeInBytes()
	}
	//contribution from the directory itself
	return sum + 4096
}

//FileModeForArchive implements the Node interface.
func (d *Directory) FileModeForArchive(includingFileType bool) uint32 {
	if includingFileType {
		return 040000 | (uint32(d.Metadata.Mode) & 07777)
	}
	return uint32(d.Metadata.Mode) & 07777
}

//Walk implements the Node interface.
func (d *Directory) Walk(absolutePath string, callback func(string, Node) error) error {
	err := callback(absolutePath, d)
	if err == filepath.SkipDir {
		return nil
	}
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
		if err != nil && err != filepath.SkipDir {
			return err
		}
	}
	return nil
}

//PostponeUnmaterializable implements the Node interface.
func (d *Directory) PostponeUnmaterializable(absolutePath string) string {
	script := d.Metadata.postponeUnmaterializable(absolutePath)

	d.Walk(absolutePath, func(path string, node Node) error {
		if node == d {
			return nil
		}
		script += node.PostponeUnmaterializable(path)
		return filepath.SkipDir
	})
	return script
}

////////////////////////////////////////////////////////////////////////////////
// RegularFile
//

//RegularFile is a type of Node that represents regular files.
type RegularFile struct {
	Content  string
	Metadata NodeMetadata
}

//Insert implements the Node interface.
func (f *RegularFile) Insert(entry Node, relPath []string, location string) error {
	if len(relPath) == 0 {
		return errors.New("duplicate entry")
	}
	return fmt.Errorf("%s is not a directory", location)
}

//InstalledSizeInBytes implements the Node interface.
func (f *RegularFile) InstalledSizeInBytes() int {
	return len(f.Content)
}

//FileModeForArchive implements the Node interface.
func (f *RegularFile) FileModeForArchive(includingFileType bool) uint32 {
	if includingFileType {
		return 0100000 | (uint32(f.Metadata.Mode) & 07777)
	}
	return uint32(f.Metadata.Mode) & 07777
}

//Walk implements the Node interface.
func (f *RegularFile) Walk(absolutePath string, callback func(string, Node) error) error {
	return callback(absolutePath, f)
}

//PostponeUnmaterializable implements the Node interface.
func (f *RegularFile) PostponeUnmaterializable(absolutePath string) string {
	return f.Metadata.postponeUnmaterializable(absolutePath)
}

//MD5Digest returns the MD5 digest of this file's contents.
func (f *RegularFile) MD5Digest() string {
	//the following is equivalent to sum := md5.Sum([]byte(f.Content)),
	//but also is backwards-compatible to Go 1.1
	digest := md5.New()
	digest.Write([]byte(f.Content))
	sum := digest.Sum(nil)

	return hex.EncodeToString(sum[:])
}

//SHA256Digest returns the SHA256 digest of this file's contents.
func (f *RegularFile) SHA256Digest() string {
	//the following is equivalent to sum := sha256.Sum([]byte(f.Content)),
	//but also is backwards-compatible to Go 1.1
	digest := sha256.New()
	digest.Write([]byte(f.Content))
	sum := digest.Sum(nil)

	return hex.EncodeToString(sum[:])
}

////////////////////////////////////////////////////////////////////////////////
// Symlink
//

//Symlink is a type of Node that represents symbolic links,
type Symlink struct {
	Target string
}

//Insert implements the Node interface.
func (s *Symlink) Insert(entry Node, relPath []string, location string) error {
	if len(relPath) == 0 {
		return errors.New("duplicate entry")
	}
	return fmt.Errorf("%s is not a directory", location)
}

//InstalledSizeInBytes implements the Node interface.
func (s *Symlink) InstalledSizeInBytes() int {
	return len(s.Target)
}

//FileModeForArchive implements the Node interface.
func (s *Symlink) FileModeForArchive(includingFileType bool) uint32 {
	if includingFileType {
		return 0120777
	}
	return 0777
}

//Walk implements the Node interface.
func (s *Symlink) Walk(absolutePath string, callback func(string, Node) error) error {
	return callback(absolutePath, s)
}

//PostponeUnmaterializable implements the Node interface.
func (s *Symlink) PostponeUnmaterializable(absolutePath string) string {
	return ""
}
