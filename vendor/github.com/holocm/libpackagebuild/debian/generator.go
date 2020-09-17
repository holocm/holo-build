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

//Package debian provides a build.Generator for Debian packages.
package debian

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	build "github.com/holocm/libpackagebuild"
	"github.com/holocm/libpackagebuild/filesystem"
)

//Generator is the build.Generator for Debian packages.
type Generator struct {
	Package *build.Package
}

//GeneratorFactory spawns Generator instances. It satisfies the build.GeneratorFactory type.
func GeneratorFactory(pkg *build.Package) build.Generator {
	return &Generator{Package: pkg}
}

var archMap = map[build.Architecture]string{
	build.ArchitectureAny:    "all",
	build.ArchitectureI386:   "i386",
	build.ArchitectureX86_64: "amd64",
	build.ArchitectureARMv5:  "armel",
	// build.ArchitectureARMv6h is not supported by Debian
	build.ArchitectureARMv7h:  "armhf",
	build.ArchitectureAArch64: "arm64",
}

//RecommendedFileName implements the build.Generator interface.
func (g *Generator) RecommendedFileName() string {
	//this is called after Build(), so we can assume that package name,
	//version, etc. were already validated
	pkg := g.Package
	return fmt.Sprintf("%s_%s_%s.deb", pkg.Name, fullVersionString(pkg), archMap[pkg.Architecture])
}

//Validate implements the build.Generator interface.
func (g *Generator) Validate() []error {
	pkg := g.Package

	//reference: https://www.debian.org/doc/debian-policy/ch-controlfields.html
	var nameRx = `[a-z0-9][a-z0-9+-.]+`
	var versionRx = `[0-9][A-Za-z0-9.+:~-]*`
	errs := pkg.ValidateWith(build.RegexSet{
		PackageName:    nameRx,
		PackageVersion: versionRx,
		RelatedName:    nameRx,
		RelatedVersion: "(?:[0-9]+:)?" + versionRx + "(?:-[1-9][0-9]*)?", //incl. release/epoch
		FormatName:     "Debian",
	}, archMap)

	if pkg.Author == "" {
		err := errors.New("The \"package.author\" field is required for Debian packages")
		errs = append(errs, err)
	}

	for _, rel := range pkg.Provides {
		if len(rel.Constraints) > 0 {
			err := fmt.Errorf("version constraints on \"Provides: %s\" are not allowed for Debian packages", rel.RelatedPackage)
			errs = append(errs, err)
		}
	}

	return errs
}

func fullVersionString(pkg *build.Package) string {
	var b strings.Builder

	if pkg.Epoch > 0 {
		fmt.Fprintf(&b, "%d:", pkg.Epoch)
	}

	b.WriteString(pkg.Version)

	if pkg.PrereleaseType != build.PrereleaseTypeNone {
		fmt.Fprintf(&b, "~%s.%d", pkg.PrereleaseType.ToString(), pkg.PrereleaseNo)
	}

	fmt.Fprintf(&b, "-%d", pkg.Release)

	return b.String()
}

type arArchiveEntry struct {
	Name string
	Data []byte
}

//Build implements the build.Generator interface.
func (g *Generator) Build() ([]byte, error) {
	pkg := g.Package
	pkg.PrepareBuild()

	//compress data.tar.xz
	var dataTar bytes.Buffer
	err := pkg.FSRoot.ToTarXZArchive(&dataTar, true, false)
	if err != nil {
		return nil, err
	}

	//prepare a directory into which to assemble the metadata files for control.tar.gz
	controlTar, err := buildControlTar(pkg)
	if err != nil {
		return nil, err
	}

	//build ar archive
	return buildArArchive([]arArchiveEntry{
		{"debian-binary", []byte("2.0\n")},
		{"control.tar.gz", controlTar},
		{"data.tar.xz", dataTar.Bytes()},
	})
}

func buildControlTar(pkg *build.Package) ([]byte, error) {
	//prepare a directory into which to put all these files
	controlDir := filesystem.NewDirectory()

	//place all the required files in there (NOTE: using the conffiles file
	//does not seem to be appropriate for our use-case, although I'll let more
	//experienced Debian users judge this one)
	err := writeControlFile(pkg, controlDir)
	if err != nil {
		return nil, err
	}
	writeMD5SumsFile(pkg, controlDir)

	//write postinst script if necessary
	script := pkg.Script(build.SetupAction)
	if script != "" {
		script := "#!/bin/bash\n" + script + "\n"
		controlDir.Entries["postinst"] = &filesystem.RegularFile{
			Content:  script,
			Metadata: filesystem.NodeMetadata{Mode: 0755},
		}
	}

	//write postrm script if necessary
	script = pkg.Script(build.CleanupAction)
	if script != "" {
		script := "#!/bin/bash\n" + script + "\n"
		controlDir.Entries["postrm"] = &filesystem.RegularFile{
			Content:  script,
			Metadata: filesystem.NodeMetadata{Mode: 0755},
		}
	}

	var buf bytes.Buffer
	err = controlDir.ToTarGZArchive(&buf, true, false)
	return buf.Bytes(), err
}

func writeControlFile(pkg *build.Package, controlDir *filesystem.Directory) error {
	//reference for this file:
	//https://www.debian.org/doc/debian-policy/ch-controlfields.html#s-binarycontrolfiles
	contents := fmt.Sprintf("Package: %s\n", pkg.Name)
	contents += fmt.Sprintf("Version: %s\n", fullVersionString(pkg))
	contents += fmt.Sprintf("Architecture: %s\n", archMap[pkg.Architecture])
	contents += fmt.Sprintf("Maintainer: %s\n", pkg.Author)
	contents += fmt.Sprintf("Installed-Size: %d\n", int(pkg.FSRoot.InstalledSizeInBytes()/1024)) // convert bytes to KiB
	contents += "Section: misc\n"
	contents += "Priority: optional\n"

	//compile relations
	rels, err := compilePackageRelations("Depends", pkg.Requires)
	if err != nil {
		return err
	}
	contents += rels

	rels, err = compilePackageRelations("Provides", pkg.Provides)
	if err != nil {
		return err
	}
	contents += rels

	rels, err = compilePackageRelations("Conflicts", pkg.Conflicts)
	if err != nil {
		return err
	}
	contents += rels

	rels, err = compilePackageRelations("Replaces", pkg.Replaces)
	if err != nil {
		return err
	}
	contents += rels

	//we have only one description field, which we use both as the synopsis and the extended description
	desc := strings.TrimSpace(strings.Replace(pkg.Description, "\n", " ", -1))
	if desc == "" {
		desc = strings.TrimSpace(pkg.Name) //description field is strictly required
	}
	contents += fmt.Sprintf("Description: %s\n %s\n", desc, desc)

	controlDir.Entries["control"] = &filesystem.RegularFile{
		Content:  contents,
		Metadata: filesystem.NodeMetadata{Mode: 0644},
	}
	return nil
}

func compilePackageRelations(relType string, rels []build.PackageRelation) (string, error) {
	if len(rels) == 0 {
		return "", nil
	}

	entries := make([]string, 0, len(rels))
	//foreach related package...
	for _, rel := range rels {
		name := rel.RelatedPackage

		//...compile constraints into a list like ">= 2.4, << 3.0" (operators "<" and ">" become "<<" and ">>" here)
		if len(rel.Constraints) > 0 {
			for _, c := range rel.Constraints {
				operator := c.Relation
				if operator == "<" {
					operator = "<<"
				}
				if operator == ">" {
					operator = ">>"
				}
				entries = append(entries, fmt.Sprintf("%s (%s %s)", name, operator, c.Version))
			}
		} else {
			entries = append(entries, name)
		}
	}

	return fmt.Sprintf("%s: %s\n", relType, strings.Join(entries, ", ")), nil
}

func writeMD5SumsFile(pkg *build.Package, controlDir *filesystem.Directory) {
	//calculate MD5 sums for all regular files in this package
	var lines []string
	pkg.WalkFSWithRelativePaths(func(path string, node filesystem.Node) error {
		file, ok := node.(*filesystem.RegularFile)
		if !ok {
			return nil //look only at regular files
		}
		lines = append(lines, fmt.Sprintf("%s  %s\n", file.MD5Digest(), path))
		return nil
	})

	controlDir.Entries["md5sums"] = &filesystem.RegularFile{
		Content:  strings.Join(lines, ""),
		Metadata: filesystem.NodeMetadata{Mode: 0644},
	}
}

func buildArArchive(entries []arArchiveEntry) ([]byte, error) {
	//we only need a very small subset of the ar archive format, so we can
	//directly construct it without requiring an extra library
	buf := bytes.NewBuffer([]byte("!<arch>\n"))

	//most fields are static
	headerFormat := "%-16s"
	headerFormat += "0           " //modification time = UNIX timestamp 0 (for reproducability)
	headerFormat += "0     "       //owner ID = root
	headerFormat += "0     "       //group ID = root
	headerFormat += "100644  "     //file mode = regular file, rw-r--r--
	headerFormat += "%-10d"        //file size in bytes
	headerFormat += "\x60\n"       //magic header separator

	for _, entry := range entries {
		fmt.Fprintf(buf, headerFormat, entry.Name, len(entry.Data))
		buf.Write(entry.Data)
		//pad data to 2-byte boundary
		if len(entry.Data)%2 == 1 {
			buf.Write([]byte{'\n'})
		}
	}

	return buf.Bytes(), nil
}
