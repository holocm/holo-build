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

package pacman

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"../common"
)

//Generator is the common.Generator for Pacman packages (as used by Arch Linux
//and derivatives).
type Generator struct{}

var archMap = map[common.Architecture]string{
	common.ArchitectureAny:     "any",
	common.ArchitectureI386:    "i686",
	common.ArchitectureX86_64:  "x86_64",
	common.ArchitectureARMv5:   "arm",
	common.ArchitectureARMv6h:  "armv6h",
	common.ArchitectureARMv7h:  "armv7h",
	common.ArchitectureAArch64: "aarch64",
}

//RecommendedFileName implements the common.Generator interface.
func (g *Generator) RecommendedFileName(pkg *common.Package) string {
	//this is called after Build(), so we can assume that package name,
	//version, etc. were already validated
	return fmt.Sprintf("%s-%s-%s.pkg.tar.xz", pkg.Name, fullVersionString(pkg), archMap[pkg.Architecture])
}

//Validate implements the common.Generator interface.
func (g *Generator) Validate(pkg *common.Package) []error {
	var nameRx = `[a-z0-9@._+][a-z0-9@._+-]*`
	var versionRx = `[a-zA-Z0-9._]+`
	return pkg.ValidateWith(common.RegexSet{
		PackageName:    nameRx,
		PackageVersion: versionRx,
		RelatedName:    "(?:except:)?(?:group:)?" + nameRx,
		RelatedVersion: "(?:[0-9]+:)?" + versionRx + "(?:-[1-9][0-9]*)?", //incl. release/epoch
		FormatName:     "pacman",
	}, archMap)
}

//Build implements the common.Generator interface.
func (g *Generator) Build(pkg *common.Package) ([]byte, error) {
	//write .PKGINFO
	err := writePKGINFO(pkg)
	if err != nil {
		return nil, fmt.Errorf("Failed to write .PKGINFO: %s", err.Error())
	}

	//write .INSTALL
	writeINSTALL(pkg)

	//write mtree
	err = writeMTREE(pkg)
	if err != nil {
		return nil, fmt.Errorf("Failed to write .MTREE: %s", err.Error())
	}

	//compress package
	return pkg.FSRoot.ToTarXZArchive(false, true)
}

func fullVersionString(pkg *common.Package) string {
	str := fmt.Sprintf("%s-%d", pkg.Version, pkg.Release)
	if pkg.Epoch > 0 {
		str = fmt.Sprintf("%d:%s", pkg.Epoch, str)
	}
	return str
}

func writePKGINFO(pkg *common.Package) error {
	//normalize package description like makepkg does
	desc := regexp.MustCompile(`\s+`).ReplaceAllString(strings.TrimSpace(pkg.Description), " ")

	//generate .PKGINFO
	contents := "# Generated by holo-build\n"
	contents += fmt.Sprintf("pkgname = %s\n", pkg.Name)
	contents += fmt.Sprintf("pkgver = %s\n", fullVersionString(pkg))
	contents += fmt.Sprintf("pkgdesc = %s\n", desc)
	contents += "url = \n"
	if pkg.Author == "" {
		contents += "packager = Unknown Packager\n"
	} else {
		contents += fmt.Sprintf("packager = %s\n", pkg.Author)
	}
	contents += fmt.Sprintf("size = %d\n", pkg.FSRoot.InstalledSizeInBytes())
	contents += fmt.Sprintf("arch = %s\n", archMap[pkg.Architecture])
	contents += "license = custom:none\n"
	replaces, err := compilePackageRequirements("replaces", pkg.Replaces)
	if err != nil {
		return err
	}
	conflicts, err := compilePackageRequirements("conflict", pkg.Conflicts)
	if err != nil {
		return err
	}
	provides, err := compilePackageRequirements("provides", pkg.Provides)
	if err != nil {
		return err
	}
	contents += replaces + conflicts + provides
	contents += compileBackupMarkers(pkg)
	requires, err := compilePackageRequirements("depend", pkg.Requires)
	if err != nil {
		return err
	}
	contents += requires

	//we used holo-build to build this, so the build depends on this package
	contents += "makedepend = holo-build\n"
	//these makepkgopt are fabricated (well, duh) and describe the behavior of
	//holo-build in terms of these options
	contents += "makepkgopt = !strip\n"
	contents += "makepkgopt = docs\n"
	contents += "makepkgopt = libtool\n"
	contents += "makepkgopt = staticlibs\n"
	contents += "makepkgopt = emptydirs\n"
	contents += "makepkgopt = !zipman\n"
	contents += "makepkgopt = !purge\n"
	contents += "makepkgopt = !upx\n"
	contents += "makepkgopt = !debug\n"

	//write .PKGINFO
	pkg.FSRoot.Entries[".PKGINFO"] = &common.FSRegularFile{
		Content:  contents,
		Metadata: common.FSNodeMetadata{Mode: 0644},
	}
	return nil
}

func compileBackupMarkers(pkg *common.Package) string {
	var lines []string
	pkg.WalkFSWithRelativePaths(func(path string, node common.FSNode) error {
		if _, ok := node.(*common.FSRegularFile); !ok {
			return nil //look only at regular files
		}
		if !strings.HasPrefix(path, "usr/share/holo/") {
			lines = append(lines, fmt.Sprintf("backup = %s\n", path))
		}
		return nil
	})
	sort.Strings(lines)
	return strings.Join(lines, "")
}

func writeINSTALL(pkg *common.Package) {
	//assemble the contents for the .INSTALL file
	contents := ""
	if script := pkg.Script(common.SetupAction); script != "" {
		contents += fmt.Sprintf("post_install() {\n%s\n}\npost_upgrade() {\npost_install\n}\n", script)
	}
	if script := pkg.Script(common.CleanupAction); script != "" {
		contents += fmt.Sprintf("post_remove() {\n%s\n}\n", script)
	}

	//do we need the .INSTALL file at all?
	if contents == "" {
		return
	}

	pkg.FSRoot.Entries[".INSTALL"] = &common.FSRegularFile{
		Content:  contents,
		Metadata: common.FSNodeMetadata{Mode: 0644},
	}
}

func writeMTREE(pkg *common.Package) error {
	contents, err := MakeMTREE(pkg)
	if err != nil {
		return err
	}

	pkg.FSRoot.Entries[".MTREE"] = &common.FSRegularFile{
		Content:  string(contents),
		Metadata: common.FSNodeMetadata{Mode: 0644},
	}
	return nil
}
