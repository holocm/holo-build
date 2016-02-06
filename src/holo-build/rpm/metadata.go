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

package rpm

import (
	"fmt"
	"path/filepath"
	"strings"

	"../common"
)

//MakeHeaderSection produces the header section of an RPM header.
func MakeHeaderSection(pkg *common.Package, payload *Payload) []byte {
	h := &Header{}

	addPackageInformationTags(h, pkg)
	h.AddInt32Value(RpmtagArchiveSize, []int32{int32(payload.UncompressedSize)})

	addInstallationTags(h, pkg)

	addFileInformationTags(h, pkg)

	//TODO: [LSB, 25.2.4.4] dependency information tags

	return h.ToBinary(RpmtagHeaderImmutable)
}

//see [LSB,25.2.4.1]
func addPackageInformationTags(h *Header, pkg *common.Package) {
	h.AddStringValue(RpmtagName, pkg.Name, false)
	h.AddStringValue(RpmtagVersion, versionString(pkg), false)
	h.AddStringValue(RpmtagRelease, fmt.Sprintf("%d", pkg.Release), false)

	//summary == first line of description
	descSplit := strings.SplitN(pkg.Description, "\n", 2)
	h.AddStringValue(RpmtagSummary, descSplit[0], true)
	h.AddStringValue(RpmtagDescription, pkg.Description, true)
	sizeInBytes := int32(pkg.FSRoot.InstalledSizeInBytes())
	h.AddInt32Value(RpmtagSize, []int32{sizeInBytes})

	//TODO validate that RPM implementations actually like this; there seems to
	//be no reference for how to spell "no license" in RPM (to compare, the
	//License attribute is optional in dpkg, and pacman accepts "custom:none")
	h.AddStringValue(RpmtagLicense, "None", false)

	if pkg.Author != "" {
		h.AddStringValue(RpmtagPackager, pkg.Author, false)
	}

	//source for valid package groups:
	//  <https://en.opensuse.org/openSUSE:Package_group_guidelines>
	//There is no such link for Fedora. Fedora treats the Group tag as optional
	//even though [LSB] says it's required. Source:
	//  <https://fedoraproject.org/wiki/Packaging:Guidelines?rd=Packaging/Guidelines#Group_tag>
	h.AddStringValue(RpmtagGroup, "System/Management", true)

	h.AddStringValue(RpmtagOs, "linux", false)
	h.AddStringValue(RpmtagArch, "noarch", false)

	h.AddStringValue(RpmtagPayloadFormat, "cpio", false)
	h.AddStringValue(RpmtagPayloadCompressor, "lzma", false)
	h.AddStringValue(RpmtagPayloadFlags, "5", false)
}

//see [LSB,25.2.4.2]
func addInstallationTags(h *Header, pkg *common.Package) {
	if pkg.SetupScript != "" {
		h.AddStringValue(RpmtagPostIn, pkg.SetupScript, false)
		h.AddStringValue(RpmtagPostInProg, "/bin/sh", false)
	}
	if pkg.CleanupScript != "" {
		h.AddStringValue(RpmtagPostUn, pkg.CleanupScript, false)
		h.AddStringValue(RpmtagPostUnProg, "/bin/sh", false)
	}
}

//see [LSB,25.2.4.3]
func addFileInformationTags(h *Header, pkg *common.Package) {
	var (
		sizes       []int32
		modes       []int16
		rdevs       []int16
		mtimes      []int32
		md5s        []string
		linktos     []string
		flags       []int32
		ownerNames  []string
		groupNames  []string
		devices     []int32
		inodes      []int32
		langs       []string
		dirIndexes  []int32
		basenames   []string
		dirnames    []string
		inodeNumber int32
	)

	//collect attributes for all files in the archive
	//(NOTE: This traversal works in the same way as the one in MakePayload.)
	pkg.WalkFSWithAbsolutePaths(func(path string, node common.FSNode) error {
		//skip implicitly created directories (as rpmbuild-constructed CPIO
		//archives apparently do)
		if n, ok := node.(*common.FSDirectory); ok {
			if n.Implicit {
				return nil
			}
		}

		//stupid stuff (which is an understatement because this whole section
		//is completely redundant)
		inodeNumber++ //make up inode numbers in the same way as rpmbuild does
		inodes = append(inodes, inodeNumber)
		langs = append(langs, "")
		devices = append(devices, 1)
		rdevs = append(rdevs, 0)

		//split path into dirname and basename
		basenames = append(basenames, filepath.Base(path))
		var dirIdx int
		dirnames, dirIdx = findOrAppend(dirnames, filepath.Dir(path))
		dirIndexes = append(dirIndexes, int32(dirIdx))

		//actually plausible metadata
		modes = append(modes, int16(node.FileModeForArchive()))
		mtimes = append(mtimes, 0)

		//type-dependent metadata
		switch n := node.(type) {
		case *common.FSDirectory:
			sizes = append(sizes, 4096)
			md5s = append(md5s, "")
			linktos = append(linktos, "")
			flags = append(flags, 0)
			ownerNames = append(ownerNames, makeUpUserOrGroupName(n.Metadata.UID(), "uid"))
			groupNames = append(groupNames, makeUpUserOrGroupName(n.Metadata.GID(), "gid"))
		case *common.FSRegularFile:
			sizes = append(sizes, int32(len(n.Content)))
			md5s = append(md5s, n.MD5Digest())
			linktos = append(linktos, "")
			flags = append(flags, RpmfileNoReplace)
			ownerNames = append(ownerNames, makeUpUserOrGroupName(n.Metadata.UID(), "uid"))
			groupNames = append(groupNames, makeUpUserOrGroupName(n.Metadata.GID(), "gid"))
		case *common.FSSymlink:
			sizes = append(sizes, int32(len(n.Target)))
			md5s = append(md5s, "")
			linktos = append(linktos, n.Target)
			flags = append(flags, 0)
			ownerNames = append(ownerNames, "root")
			groupNames = append(groupNames, "root")
		}

		return nil
	})

	h.AddInt32Value(RpmtagFileSizes, sizes)
	h.AddInt16Value(RpmtagFileModes, modes)
	h.AddInt16Value(RpmtagFileRdevs, rdevs)
	h.AddInt32Value(RpmtagFileMtimes, mtimes)
	h.AddStringArrayValue(RpmtagFileMD5s, md5s)
	h.AddStringArrayValue(RpmtagFileLinktos, linktos)
	h.AddInt32Value(RpmtagFileFlags, flags)
	h.AddStringArrayValue(RpmtagFileUserName, ownerNames)
	h.AddStringArrayValue(RpmtagFileGroupName, groupNames)
	h.AddInt32Value(RpmtagFileDevices, devices)
	h.AddInt32Value(RpmtagFileInodes, inodes)
	h.AddStringArrayValue(RpmtagFileLangs, langs)
	h.AddInt32Value(RpmtagDirIndexes, dirIndexes)
	h.AddStringArrayValue(RpmtagBasenames, basenames)
	h.AddStringArrayValue(RpmtagDirNames, dirnames)
}

//If `list` contains `value`, otherwise append `value` to `list`.
//Return the new list and the index of `value` in `list`.
func findOrAppend(list []string, value string) (newList []string, position int) {
	for idx, elem := range list {
		if elem == value {
			return list, idx
		}
	}
	length := len(list)
	return append(list, value), length
}

//Return "root" for uid/gid 0. Otherwise, just make up a name. They don't
//matter anyway since we apply name-based users/groups in the post-install
//script.
func makeUpUserOrGroupName(id uint32, prefix string) string {
	if id == 0 {
		return "root"
	}
	return prefix + fmt.Sprintf("%d", id)
}
