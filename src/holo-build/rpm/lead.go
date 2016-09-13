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
	"bytes"
	"encoding/binary"

	"../common"
)

//Lead represents the RPM lead (the first header of an RPM file, before the
//actual header sections).
type Lead struct {
	Magic              [4]byte
	Version            [2]byte
	Type               uint16
	Architecture       uint16
	NameVersionRelease [66]byte
	OperatingSystem    uint16
	SignatureType      uint16
	Reserved           [16]byte
}

//NewLead creates a lead for the given package.
func NewLead(pkg *common.Package) *Lead {
	lead := &Lead{
		Magic:        [4]byte{0xed, 0xab, 0xee, 0xdb},
		Version:      [2]byte{0x03, 0x00},
		Type:         0, //binary package
		Architecture: archIDMap[pkg.Architecture],
		//NameVersionRelease initialized below
		OperatingSystem: 1, //Linux
		SignatureType:   5, //signature section follows
		Reserved: [16]byte{
			0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00,
		},
	}

	//initialize name-version-release string, but respect limited field size
	nvr := []byte(pkg.Name + "-" + fullVersionString(pkg))
	for idx := 0; idx < 65; idx++ {
		if idx < len(nvr) {
			lead.NameVersionRelease[idx] = nvr[idx]
		} else {
			lead.NameVersionRelease[idx] = 0
		}
	}
	//must be a NUL-terminated string
	lead.NameVersionRelease[65] = 0

	return lead
}

//ToBinary returns the binary encoding for this lead.
func (l *Lead) ToBinary() []byte {
	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, l)
	return buf.Bytes()
}
