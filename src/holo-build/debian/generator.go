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

package debian

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"time"

	"../common"
)

//Generator is the common.Generator for Debian packages.
type Generator struct{}

//RecommendedFileName implements the common.Generator interface.
func (g *Generator) RecommendedFileName(pkg *common.Package) string {
	//this is called after Build(), so we can assume that package name,
	//version, etc. were already validated
	return fmt.Sprintf("%s_%s_any.deb", pkg.Name, fullVersionString(pkg))
}

//Validate implements the common.Generator interface.
func (g *Generator) Validate(pkg *common.Package) []error {
	//reference: https://www.debian.org/doc/debian-policy/ch-controlfields.html
	var nameRx = `[a-z0-9][a-z0-9+-.]+`
	var versionRx = `[0-9][A-Za-z0-9.+:~-]*`
	errs := pkg.ValidateWith(common.RegexSet{
		PackageName:    nameRx,
		PackageVersion: versionRx,
		RelatedName:    nameRx,
		RelatedVersion: "(?:[0-9]+:)?" + versionRx + "(?:-[1-9][0-9]*)?", //incl. release/epoch
		FormatName:     "Debian",
	})

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

func fullVersionString(pkg *common.Package) string {
	str := fmt.Sprintf("%s-%d", pkg.Version, pkg.Release)
	if pkg.Epoch > 0 {
		str = fmt.Sprintf("%d:%s", pkg.Epoch, str)
	}
	return str
}

type arArchiveEntry struct {
	Name string
	Data []byte
}

//Build implements the common.Generator interface.
func (g *Generator) Build(pkg *common.Package, buildReproducibly bool) ([]byte, error) {
	//compress data.tar.xz
	dataTar, err := pkg.FSRoot.ToTarXZArchive(true, false, buildReproducibly)
	if err != nil {
		return nil, err
	}

	//prepare a directory into which to assemble the metadata files for control.tar.gz
	controlTar, err := buildControlTar(pkg, buildReproducibly)
	if err != nil {
		return nil, err
	}

	//build ar archive
	return buildArArchive([]arArchiveEntry{
		arArchiveEntry{"debian-binary", []byte("2.0\n")},
		arArchiveEntry{"control.tar.gz", controlTar},
		arArchiveEntry{"data.tar.xz", dataTar},
	})
}

func buildControlTar(pkg *common.Package, buildReproducibly bool) ([]byte, error) {
	//prepare a directory into which to put all these files
	controlDir := common.NewFSDirectory()

	//place all the required files in there (NOTE: using the conffiles file
	//does not seem to be appropriate for our use-case, although I'll let more
	//experienced Debian users judge this one)
	err := writeControlFile(pkg, controlDir, buildReproducibly)
	if err != nil {
		return nil, err
	}
	writeMD5SumsFile(pkg, controlDir, buildReproducibly)

	//write postinst script if necessary
	if strings.TrimSpace(pkg.SetupScript) != "" {
		script := "#!/bin/bash\n" + strings.TrimSuffix(pkg.SetupScript, "\n") + "\n"
		controlDir.Entries["postinst"] = &common.FSRegularFile{
			Content:  script,
			Metadata: common.FSNodeMetadata{Mode: 0755},
		}
	}

	//write postrm script if necessary
	if strings.TrimSpace(pkg.CleanupScript) != "" {
		script := "#!/bin/bash\n" + strings.TrimSuffix(pkg.CleanupScript, "\n") + "\n"
		controlDir.Entries["postrm"] = &common.FSRegularFile{
			Content:  script,
			Metadata: common.FSNodeMetadata{Mode: 0755},
		}
	}

	return controlDir.ToTarGZArchive(true, false, buildReproducibly)
}

func writeControlFile(pkg *common.Package, controlDir *common.FSDirectory, buildReproducibly bool) error {
	//reference for this file:
	//https://www.debian.org/doc/debian-policy/ch-controlfields.html#s-binarycontrolfiles
	contents := fmt.Sprintf("Package: %s\n", pkg.Name)
	contents += fmt.Sprintf("Version: %s\n", fullVersionString(pkg))
	contents += "Architecture: all\n"
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

	controlDir.Entries["control"] = &common.FSRegularFile{
		Content:  contents,
		Metadata: common.FSNodeMetadata{Mode: 0644},
	}
	return nil
}

func compilePackageRelations(relType string, rels []common.PackageRelation) (string, error) {
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

func writeMD5SumsFile(pkg *common.Package, controlDir *common.FSDirectory, buildReproducibly bool) {
	//calculate MD5 sums for all regular files in this package
	var lines []string
	pkg.WalkFSWithRelativePaths(func(path string, node common.FSNode) error {
		file, ok := node.(*common.FSRegularFile)
		if !ok {
			return nil //look only at regular files
		}
		lines = append(lines, fmt.Sprintf("%s  %s\n", file.MD5Digest(), path))
		return nil
	})

	controlDir.Entries["md5sums"] = &common.FSRegularFile{
		Content:  strings.Join(lines, ""),
		Metadata: common.FSNodeMetadata{Mode: 0644},
	}
}

func buildArArchive(entries []arArchiveEntry) ([]byte, error) {
	//we only need a very small subset of the ar archive format, so we can
	//directly construct it without requiring an extra library
	buf := bytes.NewBuffer([]byte("!<arch>\n"))

	//most fields are static
	now := time.Now().Unix()
	headerFormat := "%-16s"
	headerFormat += fmt.Sprintf("%-12d", now) //modification time = now
	headerFormat += "0     "                  //owner ID = root
	headerFormat += "0     "                  //group ID = root
	headerFormat += "100644  "                //file mode = regular file, rw-r--r--
	headerFormat += "%-10d"                   //file size in bytes
	headerFormat += "\x60\n"                  //magic header separator

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
