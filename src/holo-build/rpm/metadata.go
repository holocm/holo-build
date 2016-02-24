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

	addDependencyInformationTags(h, pkg)

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
		//dirname needs a "/" suffix
		dirname := filepath.Dir(path)
		if !strings.HasSuffix(dirname, "/") {
			dirname = dirname + "/"
		}
		var dirIdx int
		dirnames, dirIdx = findOrAppend(dirnames, dirname)
		dirIndexes = append(dirIndexes, int32(dirIdx))

		//actually plausible metadata
		modes = append(modes, int16(node.FileModeForArchive(true)))
		mtimes = append(mtimes, 0)

		//type-dependent metadata
		switch n := node.(type) {
		case *common.FSDirectory:
			sizes = append(sizes, 4096)
			md5s = append(md5s, "")
			linktos = append(linktos, "")
			flags = append(flags, 0)
			ownerNames = append(ownerNames, idToString(n.Metadata.UID()))
			groupNames = append(groupNames, idToString(n.Metadata.GID()))
		case *common.FSRegularFile:
			sizes = append(sizes, int32(len(n.Content)))
			md5s = append(md5s, n.MD5Digest())
			linktos = append(linktos, "")
			flags = append(flags, RpmfileNoReplace)
			ownerNames = append(ownerNames, idToString(n.Metadata.UID()))
			groupNames = append(groupNames, idToString(n.Metadata.GID()))
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

//Convert the given UID/GID into something that's maybe suitable for a
//username/groupname field.
func idToString(id uint32) string {
	if id == 0 {
		return "root"
	}
	return fmt.Sprintf("%d", id)
}

//see [LSB,25.2.4.4]
func addDependencyInformationTags(h *Header, pkg *common.Package) {
	serializeRelations(h, pkg.Requires,
		RpmtagRequireName, RpmtagRequireFlags, RpmtagRequireVersion)
	serializeRelations(h, pkg.Provides,
		RpmtagProvideName, RpmtagProvideFlags, RpmtagProvideVersion)
	serializeRelations(h, pkg.Conflicts,
		RpmtagConflictName, RpmtagConflictFlags, RpmtagConflictVersion)
	serializeRelations(h, pkg.Replaces,
		RpmtagObsoleteName, RpmtagObsoleteFlags, RpmtagObsoleteVersion)
}

type rpmlibPseudoDependency struct {
	Name    string
	Version string
}

var rpmlibPseudoDependencies = []rpmlibPseudoDependency{
	//indicate that RPMTAG_PROVIDENAME and RPMTAG_OBSOLETENAME may have a
	//version associated with them (as if the presence of
	//RPMTAG_PROVIDEVERSION and RPMTAG_OBSOLETEVERSION is not enough)
	rpmlibPseudoDependency{"VersionedDependencies", "3.0.3-1"},
	//indicate that filenames in the payload are represented in the
	//RPMTAG_DIRINDEXES, RPMTAG_DIRNAME and RPMTAG_BASENAMES indexes
	//(again, as if the presence of these tags wasn't evidence enough)
	rpmlibPseudoDependency{"CompressedFileNames", "3.0.4-1"},
	//title says it all; apparently RPM devs haven't got the memo that you can
	//easily identify compression formats by the first few bytes
	rpmlibPseudoDependency{"PayloadIsLzma", "4.4.6-1"},
	//path names in the CPIO payload start with "./" because apparently you
	//cannot read that from the payload itself
	rpmlibPseudoDependency{"PayloadFilesHavePrefix", "4.0-1"},
}

var flagsForConstraintRelation = map[string]int32{
	"<":      RpmsenseLess,
	"<=":     RpmsenseLess | RpmsenseEqual,
	"=":      RpmsenseEqual,
	">=":     RpmsenseGreater | RpmsenseEqual,
	">":      RpmsenseGreater,
	"rpmlib": RpmsenseRpmlib | RpmsenseLess | RpmsenseEqual,
}

func serializeRelations(h *Header, rels []common.PackageRelation, namesTag, flagsTag, versionsTag uint32) {
	//for the Requires list, we need to add pseudo-dependencies to describe the
	//structure of our package (because apparently a custom key-value database
	//wasn't enough, so they built a second key-value database inside the
	//requirements array -- BRILLIANT!)
	if namesTag == RpmtagRequireName {
		for _, dep := range rpmlibPseudoDependencies {
			rels = append(rels, common.PackageRelation{
				RelatedPackage: "rpmlib(" + dep.Name + ")",
				Constraints: []common.VersionConstraint{
					common.VersionConstraint{Relation: "rpmlib", Version: dep.Version},
				},
			})
		}
	}

	//serialize relations into RPM's bizarre multi-array format
	var (
		names    []string
		flags    []int32
		versions []string
	)
	for _, rel := range rels {
		if len(rel.Constraints) == 0 {
			//case 1: no version constraints -> generate one relation for the RelatedPackage
			names = append(names, rel.RelatedPackage)
			flags = append(flags, RpmsenseAny)
			versions = append(versions, "")
		} else {
			//case 2: no version constraints -> generate one relation per constraint
			for _, cons := range rel.Constraints {
				names = append(names, rel.RelatedPackage)
				flags = append(flags, flagsForConstraintRelation[cons.Relation])
				versions = append(versions, cons.Version)
			}
		}
	}

	h.AddStringArrayValue(namesTag, names)
	h.AddInt32Value(flagsTag, flags)
	h.AddStringArrayValue(versionsTag, versions)
}
