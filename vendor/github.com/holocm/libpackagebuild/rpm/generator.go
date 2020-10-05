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

//Package rpm provides a build.Generator for RPM packages.
package rpm

import (
	"fmt"
	"strings"

	build "github.com/holocm/libpackagebuild"
)

////////////////////////////////////////////////////////////////////////////////
//
// Documentation for the RPM file format:
//
// [LSB] http://refspecs.linux-foundation.org/LSB_5.0.0/LSB-Core-generic/LSB-Core-generic/pkgformat.html
// [RPM] http://www.rpm.org/max-rpm/s1-rpm-file-format-rpm-file-format.html
//
////////////////////////////////////////////////////////////////////////////////

//Generator is the build.Generator for RPM packages.
type Generator struct {
	Package *build.Package
}

//GeneratorFactory spawns Generator instances. It satisfies the build.GeneratorFactory type.
func GeneratorFactory(pkg *build.Package) build.Generator {
	return &Generator{Package: pkg}
}

//Source for this data: `grep arch_canon /usr/lib/rpm/rpmrc`
var archMap = map[build.Architecture]string{
	build.ArchitectureAny:     "noarch",
	build.ArchitectureI386:    "i686",
	build.ArchitectureX86_64:  "x86_64",
	build.ArchitectureARMv5:   "armv5tl",
	build.ArchitectureARMv6h:  "armv6hl",
	build.ArchitectureARMv7h:  "armv7hl",
	build.ArchitectureAArch64: "aarch64",
}
var archIDMap = map[build.Architecture]uint16{
	build.ArchitectureAny:     0,
	build.ArchitectureI386:    1,
	build.ArchitectureX86_64:  1,
	build.ArchitectureARMv5:   12,
	build.ArchitectureARMv6h:  12,
	build.ArchitectureARMv7h:  12,
	build.ArchitectureAArch64: 12,
}

//Validate implements the build.Generator interface.
func (g *Generator) Validate() []error {
	//TODO, (cannot find a reliable cross-distro source of truth for the
	//acceptable format of package names and versions)
	return nil
}

//RecommendedFileName implements the build.Generator interface.
func (g *Generator) RecommendedFileName() string {
	//this is called after Build(), so we can assume that package name,
	//version, etc. were already validated
	pkg := g.Package
	return fmt.Sprintf("%s-%s.%s.rpm", pkg.Name, fullVersionString(pkg), archMap[pkg.Architecture])
}

func versionString(pkg *build.Package) string {
	var b strings.Builder

	if pkg.Epoch > 0 {
		fmt.Fprintf(&b, "%d:", pkg.Epoch)
	}

	b.WriteString(pkg.Version)

	if pkg.PrereleaseType != build.PrereleaseTypeNone {
		fmt.Fprintf(&b, "~%s.%d", pkg.PrereleaseType.String(), pkg.PrereleaseVersion)
	}

	return b.String()
}

func fullVersionString(pkg *build.Package) string {
	return fmt.Sprintf("%s-%d", versionString(pkg), pkg.Release)
}

//Build implements the build.Generator interface.
func (g *Generator) Build() ([]byte, error) {
	pkg := g.Package
	pkg.PrepareBuild()

	//assemble CPIO-LZMA payload
	payload, err := makePayload(pkg)
	if err != nil {
		return nil, err
	}

	//produce header sections in reverse order (since most of them depend on
	//what comes after them)
	headerSection := makeHeaderSection(pkg, payload)
	signatureSection := makeSignatureSection(headerSection, payload)
	lead := newLead(pkg).ToBinary()

	//combine everything with the correct alignment
	combined1 := appendAlignedTo8Byte(lead, signatureSection)
	combined2 := appendAlignedTo8Byte(combined1, headerSection)
	return append(combined2, payload.Binary...), nil
}

//According to [LSB, 25.2.2], "A Header structure shall be aligned to an 8 byte
//boundary."
func appendAlignedTo8Byte(a []byte, b []byte) []byte {
	result := a
	for len(result)%8 != 0 {
		result = append(result, 0x00)
	}
	return append(result, b...)
}
