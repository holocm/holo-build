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
	lead := writeLead(pkg)
	//TODO: signature header (write with alignment!)
	//TODO: header header (write without alignment!)
	//TODO: cpio.lzma payload
	emptyHeaderHeader := []byte{
		0x8e, 0xad, 0xe8, 0x01,
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, //no entries
		0x00, 0x00, 0x00, 0x00, //no data
	}
	emptySignatureHeader := append(emptyHeaderHeader, []byte{0x00, 0x00, 0x00, 0x00}...)
	return append(append(lead, emptySignatureHeader...), emptyHeaderHeader...), nil
}

func writeLead(pkg *common.Package) []byte {
	//pad "name-version-release" (NVR) field with NUL bytes to 66 bytes length
	nvrData := []byte(pkg.Name + "-" + fullVersionString(pkg))
	if len(nvrData) > 65 {
		nvrData = nvrData[0:65] //leave one byte at the end for trailing NUL
	}
	nvrData = append(make([]byte, 0, 66), nvrData...) //extend cap() to 66
	for len(nvrData) < 66 {
		nvrData = append(nvrData, '\000')
	}

	lead := []byte{
		//char magic[4];
		0xed, 0xab, 0xee, 0xdb,
		//unsigned char major, minor;
		0x03, 0x00,
		//short type; (0 = binary package)
		0x00, 0x00,
		//short arch; (0 = noarch)
		0x00, 0x00,
	}
	//char name[66];
	lead = append(lead, nvrData...)
	lead = append(lead, []byte{
		//short osnum; (1 = Linux)
		0x00, 0x01,
		//short signature_type; (5 = signature header)
		0x00, 0x05,
		//char reserved[16];
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00,
	}...)

	return lead
}
