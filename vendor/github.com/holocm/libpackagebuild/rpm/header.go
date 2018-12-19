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
)

//rpmHeader represents an RPM header structure (as used in the signature section
//and header section), as defined in [LSB, 25.2.2].
type rpmHeader struct {
	Records      []*rpmHeaderIndexRecord
	Data         []byte
	hasI18NTable bool
}

//rpmHeaderIndexRecord represents an index record in a RPM header structure, i.e.
//a single key-value entry. The actual value is stored in the associated
//rpmHeader.Data field. Defined in [LSB, 25.2.2.2].
type rpmHeaderIndexRecord struct {
	Tag    uint32
	Type   uint32
	Offset uint32
	Count  uint32
}

//Binary representation of the header record. [LSB,25.2.2.1]
type headerRecord struct {
	Magic            [4]byte
	Reserved         [4]byte
	IndexRecordCount uint32
	DataSize         uint32
}

//ToBinary serializes the given header.
func (hdr *rpmHeader) ToBinary(regionTag uint32) []byte {
	var buf bytes.Buffer

	//write header record
	actualDataSize := uint32(len(hdr.Data))
	actualRecordCount := uint32(len(hdr.Records))
	binary.Write(&buf, binary.BigEndian, &headerRecord{
		Magic:            [4]byte{0x8E, 0xAD, 0xE8, 0x01},
		Reserved:         [4]byte{0x00, 0x00, 0x00, 0x00},
		IndexRecordCount: actualRecordCount + 1, //+1 for the region tag
		DataSize:         actualDataSize + 16,   //+16 for the region tag
	})

	//write index record for region tag
	//
	//A "region" is defined nowhere in any kind of spec that I could find for
	//RPM (i.e. neither in [LSB] nor [RPM]), but it's mentioned in the code of
	//the rpm-org implementation where they have some validations for it.
	//
	//I don't fully grasp the meaning, but it appears that a region tag marks
	//a set of header tags and data that are to be considered immutable, i.e.
	//they may be used for validation purposes, such as calculating hash
	//digests and signatures. I hope that I'm wrong, because this would imply
	//that RPM has a concept of "metadata that's okay to manipulate even if the
	//package is GPG-signed", which is insane even for RPM's standards.
	//However, from what I've seen in the implementation, their regions always
	//seem to span the whole header structure, therefore marking everything as
	//immutable.
	//
	//We do the same thing. The index record for the region tag is at the
	//*start* of the index record array, and its data is located at the *end*
	//of the data area. The data is another index record that (using a negative
	//offset into the data area) points back at the original index record. (I'm
	//tempted to use the word "insane", but it feels like I use that word so
	//often when talking about RPM that it has lost all meaning.)
	binary.Write(&buf, binary.BigEndian, &rpmHeaderIndexRecord{
		Tag:    regionTag,
		Type:   rpmBinType,
		Offset: actualDataSize,
		Count:  16,
	})

	//write the actual index records
	for _, ir := range hdr.Records {
		binary.Write(&buf, binary.BigEndian, ir)
	}

	//write data
	buf.Write(hdr.Data)

	//write data for the region tag (see the wall of text above)
	binary.Write(&buf, binary.BigEndian, &rpmHeaderIndexRecord{
		Tag:    regionTag,
		Type:   rpmBinType,
		Offset: -(actualRecordCount + 1) * 16,
		Count:  16,
	})

	return buf.Bytes()
}

//AddBinaryValue adds a value of type rpmBinType to this header.
func (hdr *rpmHeader) AddBinaryValue(tag uint32, data []byte) {
	hdr.Records = append(hdr.Records, &rpmHeaderIndexRecord{
		Tag:    tag,
		Type:   rpmBinType,
		Offset: uint32(len(hdr.Data)),
		Count:  uint32(len(data)),
	})
	hdr.Data = append(hdr.Data, data...)
}

//AddInt16Value adds a value of type rpmInt32Type to this header.
func (hdr *rpmHeader) AddInt16Value(tag uint32, data []int16) {
	//see near start of AddStringArrayValue() for rationale
	if len(data) == 0 {
		return
	}

	//align to 2 bytes
	if len(hdr.Data)%2 != 0 {
		hdr.Data = append(hdr.Data, 0x00)
	}

	hdr.Records = append(hdr.Records, &rpmHeaderIndexRecord{
		Tag:    tag,
		Type:   rpmInt16Type,
		Offset: uint32(len(hdr.Data)),
		Count:  uint32(len(data)),
	})
	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, data)
	hdr.Data = append(hdr.Data, buf.Bytes()...)
}

//AddInt32Value adds a value of type rpmInt32Type to this header.
func (hdr *rpmHeader) AddInt32Value(tag uint32, data []int32) {
	//see near start of AddStringArrayValue() for rationale
	if len(data) == 0 {
		return
	}

	//align to 4 bytes
	for len(hdr.Data)%4 != 0 {
		hdr.Data = append(hdr.Data, 0x00)
	}

	hdr.Records = append(hdr.Records, &rpmHeaderIndexRecord{
		Tag:    tag,
		Type:   rpmInt32Type,
		Offset: uint32(len(hdr.Data)),
		Count:  uint32(len(data)),
	})
	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, data)
	hdr.Data = append(hdr.Data, buf.Bytes()...)
}

//AddStringValue adds a value of type rpmStringType or rpmI18NStringType to
//this header.
func (hdr *rpmHeader) AddStringValue(tag uint32, data string, i18n bool) {
	var recordType uint32 = rpmStringType
	if i18n {
		recordType = rpmI18NStringType
		//I18N strings require an I18N table listing the available locales;
		//initialize that if needed
		if !hdr.hasI18NTable {
			hdr.AddStringArrayValue(rpmtagHeaderI18NTable, []string{"C"})
			hdr.hasI18NTable = true
		}
	}

	hdr.Records = append(hdr.Records, &rpmHeaderIndexRecord{
		Tag:    tag,
		Type:   recordType,
		Offset: uint32(len(hdr.Data)),
		Count:  1,
	})
	hdr.Data = append(append(hdr.Data, []byte(data)...), 0x00)
}

//AddStringArrayValue adds a value of type rpmStringArrayType to this header.
func (hdr *rpmHeader) AddStringArrayValue(tag uint32, data []string) {
	//skip the tag entirely if it does not contain any data (even if the tag
	//may be listed as "required"); e.g. in metapackages without filesystem
	//entries, none of the filesystem metadata tags may be written (this is
	//because RPM's dataLength() function returns failure if 0 strings were
	//read, even if there were legitimately 0 strings)
	if len(data) == 0 {
		return
	}

	hdr.Records = append(hdr.Records, &rpmHeaderIndexRecord{
		Tag:    tag,
		Type:   rpmStringArrayType,
		Offset: uint32(len(hdr.Data)),
		Count:  uint32(len(data)),
	})
	for _, str := range data {
		hdr.Data = append(append(hdr.Data, []byte(str)...), 0x00)
	}
}

//List of known values for rpmHeaderIndexRecord.Type. [LSB,25.2.2.2.1]
//
//Note that we don't support writing all types; null, char and int{8,16,64}
//are not needed for the tags that we must support.
const (
	rpmNullType        = 0
	rpmCharType        = 1
	rpmInt8Type        = 2
	rpmInt16Type       = 3
	rpmInt32Type       = 4
	rpmInt64Type       = 5 //reserved
	rpmStringType      = 6
	rpmBinType         = 7
	rpmStringArrayType = 8
	rpmI18NStringType  = 9
)

//List of known values for rpmHeaderIndexRecord.Tag. [LSB, 25.2.2.2.2 ff.]
const (
	rpmtagHeaderSignatures  = 62   //type: BIN
	rpmtagHeaderImmutable   = 63   //type: BIN
	rpmtagHeaderI18NTable   = 100  //type: STRING_ARRAY
	rpmsigtagSize           = 1000 //type: INT32
	rpmsigtagPayloadSize    = 1007 //type: INT32
	rpmsigtagSHA1           = 269  //type: STRING
	rpmsigtagMD5            = 1004 //type: BIN
	rpmsigtagDSA            = 267  //type: BIN
	rpmsigtagRSA            = 268  //type: BIN
	rpmsigtagPGP            = 1002 //type: BIN
	rpmsigtagGPG            = 1005 //type: BIN
	rpmtagName              = 1000 //type: STRING
	rpmtagVersion           = 1001 //type: STRING
	rpmtagRelease           = 1002 //type: STRING
	rpmtagSummary           = 1004 //type: I18NSTRING
	rpmtagDescription       = 1005 //type: I18NSTRING
	rpmtagSize              = 1009 //type: INT32
	rpmtagDistribution      = 1010 //type: STRING
	rpmtagVendor            = 1011 //type: STRING
	rpmtagLicense           = 1014 //type: STRING
	rpmtagPackager          = 1015 //type: STRING
	rpmtagGroup             = 1016 //type: I18NSTRING
	rpmtagURL               = 1020 //type: STRING
	rpmtagOs                = 1021 //type: STRING
	rpmtagArch              = 1022 //type: STRING
	rpmtagSourceRPM         = 1044 //type: STRING
	rpmtagArchiveSize       = 1046 //type: INT32
	rpmtagRPMVersion        = 1064 //type: STRING
	rpmtagCookie            = 1094 //type: STRING
	rpmtagDistURL           = 1123 //type: STRING
	rpmtagPayloadFormat     = 1124 //type: STRING
	rpmtagPayloadCompressor = 1125 //type: STRING
	rpmtagPayloadFlags      = 1126 //type: STRING
	rpmtagPreIn             = 1023 //type: STRING
	rpmtagPostIn            = 1024 //type: STRING
	rpmtagPreUn             = 1025 //type: STRING
	rpmtagPostUn            = 1026 //type: STRING
	rpmtagPreInProg         = 1085 //type: STRING
	rpmtagPostInProg        = 1086 //type: STRING
	rpmtagPreUnProg         = 1087 //type: STRING
	rpmtagPostUnProg        = 1088 //type: STRING
	rpmtagOldFileNames      = 1027 //type: STRING_ARRAY
	rpmtagFileSizes         = 1028 //type: INT32
	rpmtagFileModes         = 1030 //type: INT16
	rpmtagFileRdevs         = 1033 //type: INT16
	rpmtagFileMtimes        = 1034 //type: INT32
	rpmtagFileMD5s          = 1035 //type: STRING_ARRAY
	rpmtagFileLinktos       = 1036 //type: STRING_ARRAY
	rpmtagFileFlags         = 1037 //type: INT32
	rpmtagFileUserName      = 1039 //type: STRING_ARRAY
	rpmtagFileGroupName     = 1040 //type: STRING_ARRAY
	rpmtagFileDevices       = 1095 //type: INT32
	rpmtagFileInodes        = 1096 //type: INT32
	rpmtagFileLangs         = 1097 //type: STRING_ARRAY
	rpmtagDirIndexes        = 1116 //type: INT32
	rpmtagBasenames         = 1117 //type: STRING_ARRAY
	rpmtagDirNames          = 1118 //type: STRING_ARRAY
	rpmtagProvideName       = 1047 //type: STRING_ARRAY
	rpmtagProvideFlags      = 1112 //type: INT32
	rpmtagProvideVersion    = 1113 //type: STRING_ARRAY
	rpmtagRequireName       = 1049 //type: STRING_ARRAY
	rpmtagRequireFlags      = 1048 //type: INT32
	rpmtagRequireVersion    = 1050 //type: STRING_ARRAY
	rpmtagConflictName      = 1054 //type: STRING_ARRAY
	rpmtagConflictFlags     = 1053 //type: INT32
	rpmtagConflictVersion   = 1055 //type: STRING_ARRAY
	rpmtagObsoleteName      = 1090 //type: STRING_ARRAY
	rpmtagObsoleteFlags     = 1114 //type: INT32
	rpmtagObsoleteVersion   = 1115 //type: STRING_ARRAY
)

//Values for rpmtagFileFlags, see [LSB,25.2.4.3.1].
const (
	rpmfileConfig    = (1 << 0)
	rpmfileDoc       = (1 << 1)
	rpmfileDoNotUse  = (1 << 2)
	rpmfileMissingOK = (1 << 3)
	rpmfileNoReplace = (1 << 4)
	rpmfileSpecFile  = (1 << 5)
	rpmfileGhost     = (1 << 6)
	rpmfileLicense   = (1 << 7)
	rpmfileReadme    = (1 << 8)
	rpmfileExclude   = (1 << 9)
)

//Values for rpmtagRequireFlags, rpmtagConflictFlags, rpmtagProvideFlags, rpmtagObsoleteFlags. See [LSB,25.2.4.4.2].
//
//Note that "RPMSENSE" is copied from the spec, but is clearly a euphemism.
//There is nothing in RPM that makes sense.
const (
	rpmsenseAny          = 0
	rpmsenseLess         = 0x02
	rpmsenseGreater      = 0x04
	rpmsenseEqual        = 0x08
	rpmsensePrereq       = 0x40
	rpmsenseInterp       = 0x100
	rpmsenseScriptPre    = 0x200
	rpmsenseScriptPost   = 0x400
	rpmsenseScriptPreUn  = 0x800
	rpmsenseScriptPostUn = 0x1000
	rpmsenseRpmlib       = 0x1000000
)
