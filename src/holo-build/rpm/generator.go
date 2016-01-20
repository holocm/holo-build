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
// [LSB] http://refspecs.linux-foundation.org/LSB_3.1.0/LSB-Core-generic/LSB-Core-generic/pkgformat.html
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

func fullVersionString(pkg *common.Package) string {
	str := fmt.Sprintf("%s-%d", pkg.Version, pkg.Release)
	if pkg.Epoch > 0 {
		str = fmt.Sprintf("%d:%s", pkg.Epoch, str)
	}
	return str
}

//Build implements the common.Generator interface.
func (g *Generator) Build(pkg *common.Package, buildReproducibly bool) ([]byte, error) {
	payload, err := MakePayload(pkg, buildReproducibly)
	if err != nil {
		return nil, err
	}

	lead := NewLead(pkg).ToBinary()
	//TODO: signature header (write with alignment!)
	//TODO: header header (write without alignment!)
	emptyHeaderHeader := []byte{
		0x8e, 0xad, 0xe8, 0x01,
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, //no entries
		0x00, 0x00, 0x00, 0x00, //no data
	}
	emptySignatureHeader := append(emptyHeaderHeader, []byte{0x00, 0x00, 0x00, 0x00}...)
	return append(append(append(lead, emptySignatureHeader...), emptyHeaderHeader...), payload.Binary...), nil
}
