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
import (
	"errors"
	"fmt"
	"io/ioutil"
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
	//Materialize generates an actual filesystem entry for this node at the
	//given path.
	Materialize(path string) error
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
//
//This function assumes that, if there exist unmaterializable metadata,
//PostponeUnmaterializable() has already been called.
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
			return errors.New("duplicate entry")
		}
		d.Entries[subname] = entry
		return nil
	}

	//entry is inside a subdirectory of this one -> spawn the next child if
	//necessary and recurse
	if subentry == nil {
		//TODO: track which directories were created implicitly, and suppress the "duplicate entry" error there (the one directly above)
		subentry = NewFSDirectory()
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

//Materialize implements the FSNode interface.
func (d *FSDirectory) Materialize(path string) error {
	err := os.Mkdir(path, d.Metadata.Mode)
	if err != nil {
		return err
	}
	err = d.Metadata.ApplyTo(path)
	if err != nil {
		return err
	}
	for name, entry := range d.Entries {
		err = entry.Materialize(path + "/" + name)
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

//Walk implements the FSNode interface.
func (f *FSRegularFile) Walk(absolutePath string, callback func(string, FSNode) error) error {
	return callback(absolutePath, f)
}

//Materialize implements the FSNode interface.
func (f *FSRegularFile) Materialize(path string) error {
	err := ioutil.WriteFile(path, []byte(f.Content), f.Metadata.Mode)
	if err != nil {
		return err
	}
	return f.Metadata.ApplyTo(path)
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

//Walk implements the FSNode interface.
func (s *FSSymlink) Walk(absolutePath string, callback func(string, FSNode) error) error {
	return callback(absolutePath, s)
}

//Materialize implements the FSNode interface.
func (s *FSSymlink) Materialize(path string) error {
	return os.Symlink(s.Target, path)
}
