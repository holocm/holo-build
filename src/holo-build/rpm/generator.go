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

	"../common"
)

////////////////////////////////////////////////////////////////////////////////
//
// Documentation for the RPM file format:
//
// [LSB] http://refspecs.linux-foundation.org/LSB_5.0.0/LSB-Core-generic/LSB-Core-generic/pkgformat.html
// [RPM] http://www.rpm.org/max-rpm/s1-rpm-file-format-rpm-file-format.html
//
////////////////////////////////////////////////////////////////////////////////

//Generator is the common.Generator for RPM packages.
type Generator struct{}

//Validate implements the common.Generator interface.
func (g *Generator) Validate(pkg *common.Package) []error {
	//TODO
	return nil
}

//RecommendedFileName implements the common.Generator interface.
func (g *Generator) RecommendedFileName(pkg *common.Package) string {
	//this is called after Build(), so we can assume that package name,
	//version, etc. were already validated
	return fmt.Sprintf("%s-%s.noarch.rpm", pkg.Name, fullVersionString(pkg))
}

func versionString(pkg *common.Package) string {
	if pkg.Epoch > 0 {
		return fmt.Sprintf("%d:%s", pkg.Epoch, pkg.Version)
	}
	return pkg.Version
}

func fullVersionString(pkg *common.Package) string {
	return fmt.Sprintf("%s-%d", versionString(pkg), pkg.Release)
}

//Build implements the common.Generator interface.
func (g *Generator) Build(pkg *common.Package, buildReproducibly bool) ([]byte, error) {
	//assemble CPIO-LZMA payload
	payload, err := MakePayload(pkg, buildReproducibly)
	if err != nil {
		return nil, err
	}

	//produce header sections in reverse order (since most of them depend on
	//what comes after them)
	headerSection := MakeHeaderSection(pkg, payload)
	signatureSection := MakeSignatureSection(headerSection, payload)
	lead := NewLead(pkg).ToBinary()

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
