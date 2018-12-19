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

package rpm

import (
	"fmt"
	"path/filepath"
	"strings"

	build "github.com/holocm/libpackagebuild"
	"github.com/holocm/libpackagebuild/filesystem"
)

//makeHeaderSection produces the header section of an RPM header.
func makeHeaderSection(pkg *build.Package, payload *rpmPayload) []byte {
	h := &rpmHeader{}

	addPackageInformationTags(h, pkg)
	h.AddInt32Value(rpmtagArchiveSize, []int32{int32(payload.UncompressedSize)})

	addInstallationTags(h, pkg)

	addFileInformationTags(h, pkg)

	addDependencyInformationTags(h, pkg)

	return h.ToBinary(rpmtagHeaderImmutable)
}

//see [LSB,25.2.4.1]
func addPackageInformationTags(h *rpmHeader, pkg *build.Package) {
	h.AddStringValue(rpmtagName, pkg.Name, false)
	h.AddStringValue(rpmtagVersion, versionString(pkg), false)
	h.AddStringValue(rpmtagRelease, fmt.Sprintf("%d", pkg.Release), false)

	//summary == first line of description
	descSplit := strings.SplitN(pkg.Description, "\n", 2)
	h.AddStringValue(rpmtagSummary, descSplit[0], true)
	h.AddStringValue(rpmtagDescription, pkg.Description, true)
	sizeInBytes := int32(pkg.FSRoot.InstalledSizeInBytes())
	h.AddInt32Value(rpmtagSize, []int32{sizeInBytes})

	h.AddStringValue(rpmtagLicense, "None", false)

	if pkg.Author != "" {
		h.AddStringValue(rpmtagPackager, pkg.Author, false)
	}

	//source for valid package groups:
	//  <https://en.opensuse.org/openSUSE:Package_group_guidelines>
	//There is no such link for Fedora. Fedora treats the Group tag as optional
	//even though [LSB] says it's required. Source:
	//  <https://fedoraproject.org/wiki/Packaging:Guidelines?rd=Packaging/Guidelines#Group_tag>
	h.AddStringValue(rpmtagGroup, "System/Management", true)

	h.AddStringValue(rpmtagOs, "linux", false)
	h.AddStringValue(rpmtagArch, archMap[pkg.Architecture], false)

	h.AddStringValue(rpmtagPayloadFormat, "cpio", false)
	h.AddStringValue(rpmtagPayloadCompressor, "lzma", false)
	h.AddStringValue(rpmtagPayloadFlags, "5", false)
}

//see [LSB,25.2.4.2]
func addInstallationTags(h *rpmHeader, pkg *build.Package) {
	if script := pkg.Script(build.SetupAction); script != "" {
		h.AddStringValue(rpmtagPostIn, script, false)
		h.AddStringValue(rpmtagPostInProg, "/bin/sh", false)
	}
	if script := pkg.Script(build.CleanupAction); script != "" {
		h.AddStringValue(rpmtagPostUn, script, false)
		h.AddStringValue(rpmtagPostUnProg, "/bin/sh", false)
	}
}

//see [LSB,25.2.4.3]
func addFileInformationTags(h *rpmHeader, pkg *build.Package) {
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
	pkg.WalkFSWithAbsolutePaths(func(path string, node filesystem.Node) error {
		//skip implicitly created directories (as rpmbuild-constructed CPIO
		//archives apparently do)
		if n, ok := node.(*filesystem.Directory); ok {
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
		case *filesystem.Directory:
			sizes = append(sizes, 4096)
			md5s = append(md5s, "")
			linktos = append(linktos, "")
			flags = append(flags, 0)
			ownerNames = append(ownerNames, idToString(n.Metadata.UID()))
			groupNames = append(groupNames, idToString(n.Metadata.GID()))
		case *filesystem.RegularFile:
			sizes = append(sizes, int32(len(n.Content)))
			md5s = append(md5s, n.MD5Digest())
			linktos = append(linktos, "")
			flags = append(flags, rpmfileNoReplace)
			ownerNames = append(ownerNames, idToString(n.Metadata.UID()))
			groupNames = append(groupNames, idToString(n.Metadata.GID()))
		case *filesystem.Symlink:
			sizes = append(sizes, int32(len(n.Target)))
			md5s = append(md5s, "")
			linktos = append(linktos, n.Target)
			flags = append(flags, 0)
			ownerNames = append(ownerNames, "root")
			groupNames = append(groupNames, "root")
		}

		return nil
	})

	h.AddInt32Value(rpmtagFileSizes, sizes)
	h.AddInt16Value(rpmtagFileModes, modes)
	h.AddInt16Value(rpmtagFileRdevs, rdevs)
	h.AddInt32Value(rpmtagFileMtimes, mtimes)
	h.AddStringArrayValue(rpmtagFileMD5s, md5s)
	h.AddStringArrayValue(rpmtagFileLinktos, linktos)
	h.AddInt32Value(rpmtagFileFlags, flags)
	h.AddStringArrayValue(rpmtagFileUserName, ownerNames)
	h.AddStringArrayValue(rpmtagFileGroupName, groupNames)
	h.AddInt32Value(rpmtagFileDevices, devices)
	h.AddInt32Value(rpmtagFileInodes, inodes)
	h.AddStringArrayValue(rpmtagFileLangs, langs)
	h.AddInt32Value(rpmtagDirIndexes, dirIndexes)
	h.AddStringArrayValue(rpmtagBasenames, basenames)
	h.AddStringArrayValue(rpmtagDirNames, dirnames)
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
func addDependencyInformationTags(h *rpmHeader, pkg *build.Package) {
	serializeRelations(h, pkg.Requires,
		rpmtagRequireName, rpmtagRequireFlags, rpmtagRequireVersion)
	serializeRelations(h, pkg.Provides,
		rpmtagProvideName, rpmtagProvideFlags, rpmtagProvideVersion)
	serializeRelations(h, pkg.Conflicts,
		rpmtagConflictName, rpmtagConflictFlags, rpmtagConflictVersion)
	serializeRelations(h, pkg.Replaces,
		rpmtagObsoleteName, rpmtagObsoleteFlags, rpmtagObsoleteVersion)
}

type rpmlibPseudoDependency struct {
	Name    string
	Version string
}

var rpmlibPseudoDependencies = []rpmlibPseudoDependency{
	//indicate that RPMTAG_PROVIDENAME and RPMTAG_OBSOLETENAME may have a
	//version associated with them (as if the presence of
	//RPMTAG_PROVIDEVERSION and RPMTAG_OBSOLETEVERSION is not enough)
	{"VersionedDependencies", "3.0.3-1"},
	//indicate that filenames in the payload are represented in the
	//RPMTAG_DIRINDEXES, RPMTAG_DIRNAME and RPMTAG_BASENAMES indexes
	//(again, as if the presence of these tags wasn't evidence enough)
	{"CompressedFileNames", "3.0.4-1"},
	//title says it all; apparently RPM devs haven't got the memo that you can
	//easily identify compression formats by the first few bytes
	{"PayloadIsLzma", "4.4.6-1"},
	//path names in the CPIO payload start with "./" because apparently you
	//cannot read that from the payload itself
	{"PayloadFilesHavePrefix", "4.0-1"},
}

var flagsForConstraintRelation = map[string]int32{
	"<":      rpmsenseLess,
	"<=":     rpmsenseLess | rpmsenseEqual,
	"=":      rpmsenseEqual,
	">=":     rpmsenseGreater | rpmsenseEqual,
	">":      rpmsenseGreater,
	"rpmlib": rpmsenseRpmlib | rpmsenseLess | rpmsenseEqual,
}

func serializeRelations(h *rpmHeader, rels []build.PackageRelation, namesTag, flagsTag, versionsTag uint32) {
	//for the Requires list, we need to add pseudo-dependencies to describe the
	//structure of our package (because apparently a custom key-value database
	//wasn't enough, so they built a second key-value database inside the
	//requirements array -- BRILLIANT!)
	if namesTag == rpmtagRequireName {
		for _, dep := range rpmlibPseudoDependencies {
			rels = append(rels, build.PackageRelation{
				RelatedPackage: "rpmlib(" + dep.Name + ")",
				Constraints: []build.VersionConstraint{
					{Relation: "rpmlib", Version: dep.Version},
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
			flags = append(flags, rpmsenseAny)
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
