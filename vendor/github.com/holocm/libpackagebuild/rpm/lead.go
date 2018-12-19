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
	"bytes"
	"encoding/binary"

	build "github.com/holocm/libpackagebuild"
)

//rpmLead represents the RPM lead (the first header of an RPM file, before the
//actual header sections).
type rpmLead struct {
	Magic              [4]byte
	Version            [2]byte
	Type               uint16
	Architecture       uint16
	NameVersionRelease [66]byte
	OperatingSystem    uint16
	SignatureType      uint16
	Reserved           [16]byte
}

//newLead creates a lead for the given package.
func newLead(pkg *build.Package) *rpmLead {
	lead := &rpmLead{
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
func (l *rpmLead) ToBinary() []byte {
	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, l)
	return buf.Bytes()
}
