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

//Header represents an RPM header structure (as used in the signature section
//and header section), as defined in [LSB, 25.2.2].
type Header struct {
	Records      []*HeaderIndexRecord
	Data         []byte
	hasI18NTable bool
}

//HeaderIndexRecord represents an index record in a RPM header structure, i.e.
//a single key-value entry. The actual value is stored in the associated
//Header.Data field. Defined in [LSB, 25.2.2.2].
type HeaderIndexRecord struct {
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
func (hdr *Header) ToBinary(regionTag uint32) []byte {
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
	binary.Write(&buf, binary.BigEndian, &HeaderIndexRecord{
		Tag:    regionTag,
		Type:   RpmBinType,
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
	binary.Write(&buf, binary.BigEndian, &HeaderIndexRecord{
		Tag:    regionTag,
		Type:   RpmBinType,
		Offset: -(actualRecordCount + 1) * 16,
		Count:  16,
	})

	return buf.Bytes()
}

//AddBinaryValue adds a value of type RpmBinType to this header.
func (hdr *Header) AddBinaryValue(tag uint32, data []byte) {
	hdr.Records = append(hdr.Records, &HeaderIndexRecord{
		Tag:    tag,
		Type:   RpmBinType,
		Offset: uint32(len(hdr.Data)),
		Count:  uint32(len(data)),
	})
	hdr.Data = append(hdr.Data, data...)
}

//AddInt32Value adds a value of type RpmInt32Type to this header.
func (hdr *Header) AddInt32Value(tag uint32, data []int32) {
	//align to 4 bytes
	for len(hdr.Data)%4 != 0 {
		hdr.Data = append(hdr.Data, 0x00)
	}

	hdr.Records = append(hdr.Records, &HeaderIndexRecord{
		Tag:    tag,
		Type:   RpmInt32Type,
		Offset: uint32(len(hdr.Data)),
		Count:  uint32(len(data)),
	})
	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, data)
	hdr.Data = append(hdr.Data, buf.Bytes()...)
}

//AddStringValue adds a value of type RpmStringType or RpmI18NStringType to
//this header.
func (hdr *Header) AddStringValue(tag uint32, data string, i18n bool) {
	var recordType uint32 = RpmStringType
	if i18n {
		recordType = RpmI18NStringType
		//I18N strings require an I18N table listing the available locales;
		//initialize that if needed
		if !hdr.hasI18NTable {
			hdr.AddStringArrayValue(RpmtagHeaderI18NTable, []string{"C"})
			hdr.hasI18NTable = true
		}
	}

	hdr.Records = append(hdr.Records, &HeaderIndexRecord{
		Tag:    tag,
		Type:   recordType,
		Offset: uint32(len(hdr.Data)),
		Count:  1,
	})
	hdr.Data = append(append(hdr.Data, []byte(data)...), 0x00)
}

//AddStringArrayValue adds a value of type RpmStringArrayType to this header.
func (hdr *Header) AddStringArrayValue(tag uint32, data []string) {
	hdr.Records = append(hdr.Records, &HeaderIndexRecord{
		Tag:    tag,
		Type:   RpmStringArrayType,
		Offset: uint32(len(hdr.Data)),
		Count:  uint32(len(data)),
	})
	for _, str := range data {
		hdr.Data = append(append(hdr.Data, []byte(str)...), 0x00)
	}
}

//List of known values for HeaderIndexRecord.Type. [LSB,25.2.2.2.1]
//
//Note that we don't support writing all types; null, char and int{8,16,64}
//are not needed for the tags that we must support.
const (
	RpmNullType        = 0
	RpmCharType        = 1
	RpmInt8Type        = 2
	RpmInt16Type       = 3
	RpmInt32Type       = 4
	RpmInt64Type       = 5 //reserved
	RpmStringType      = 6
	RpmBinType         = 7
	RpmStringArrayType = 8
	RpmI18NStringType  = 9
)

//List of known values for HeaderIndexRecord.Tag. [LSB, 25.2.2.2.2 ff.]
const (
	RpmtagHeaderSignatures  = 62   //type: BIN
	RpmtagHeaderImmutable   = 63   //type: BIN
	RpmtagHeaderI18NTable   = 100  //type: STRING_ARRAY
	RpmsigtagSize           = 1000 //type: INT32
	RpmsigtagPayloadSize    = 1007 //type: INT32
	RpmsigtagSHA1           = 269  //type: STRING
	RpmsigtagMD5            = 1004 //type: BIN
	RpmsigtagDSA            = 267  //type: BIN
	RpmsigtagRSA            = 268  //type: BIN
	RpmsigtagPGP            = 1002 //type: BIN
	RpmsigtagGPG            = 1005 //type: BIN
	RpmtagName              = 1000 //type: STRING
	RpmtagVersion           = 1001 //type: STRING
	RpmtagRelease           = 1002 //type: STRING
	RpmtagSummary           = 1004 //type: I18NSTRING
	RpmtagDescription       = 1005 //type: I18NSTRING
	RpmtagSize              = 1009 //type: INT32
	RpmtagDistribution      = 1010 //type: STRING
	RpmtagVendor            = 1011 //type: STRING
	RpmtagLicense           = 1014 //type: STRING
	RpmtagPackager          = 1015 //type: STRING
	RpmtagGroup             = 1016 //type: I18NSTRING
	RpmtagURL               = 1020 //type: STRING
	RpmtagOs                = 1021 //type: STRING
	RpmtagArch              = 1022 //type: STRING
	RpmtagSourceRPM         = 1044 //type: STRING
	RpmtagArchiveSize       = 1046 //type: INT32
	RpmtagRPMVersion        = 1064 //type: STRING
	RpmtagCookie            = 1094 //type: STRING
	RpmtagDistURL           = 1123 //type: STRING
	RpmtagPayloadFormat     = 1124 //type: STRING
	RpmtagPayloadCompressor = 1125 //type: STRING
	RpmtagPayloadFlags      = 1126 //type: STRING
	RpmtagPreIn             = 1023 //type: STRING
	RpmtagPostIn            = 1024 //type: STRING
	RpmtagPreUn             = 1025 //type: STRING
	RpmtagPostUn            = 1026 //type: STRING
	RpmtagPreInProg         = 1085 //type: STRING
	RpmtagPostInProg        = 1086 //type: STRING
	RpmtagPreUnProg         = 1087 //type: STRING
	RpmtagPostUnProg        = 1088 //type: STRING
	RpmtagOldFileNames      = 1027 //type: STRING_ARRAY
	RpmtagFileSizes         = 1028 //type: INT32
	RpmtagFileModes         = 1030 //type: INT16
	RpmtagFileRdevs         = 1033 //type: INT16
	RpmtagFileMtimes        = 1034 //type: INT32
	RpmtagFileMD5s          = 1035 //type: STRING_ARRAY
	RpmtagFileLinktos       = 1036 //type: STRING_ARRAY
	RpmtagFileFlags         = 1037 //type: INT32
	RpmtagFileUserName      = 1039 //type: STRING_ARRAY
	RpmtagFileGroupName     = 1040 //type: STRING_ARRAY
	RpmtagFileDevices       = 1095 //type: INT32
	RpmtagFileInodes        = 1096 //type: INT32
	RpmtagFileLangs         = 1097 //type: STRING_ARRAY
	RpmtagDirIndexes        = 1116 //type: INT32
	RpmtagBasenames         = 1117 //type: STRING_ARRAY
	RpmtagDirNames          = 1118 //type: STRING_ARRAY
	RpmtagProvideName       = 1047 //type: STRING_ARRAY
	RpmtagProvideFlags      = 1112 //type: INT32
	RpmtagProvideVersion    = 1113 //type: STRING_ARRAY
	RpmtagRequireName       = 1049 //type: STRING_ARRAY
	RpmtagRequireFlags      = 1048 //type: INT32
	RpmtagRequireVersion    = 1050 //type: STRING_ARRAY
	RpmtagConflictName      = 1054 //type: STRING_ARRAY
	RpmtagConflictFlags     = 1053 //type: INT32
	RpmtagConflictVersion   = 1055 //type: STRING_ARRAY
	RpmtagObsoleteName      = 1090 //type: STRING_ARRAY
	RpmtagObsoleteFlags     = 1114 //type: INT32
	RpmtagObsoleteVersion   = 1115 //type: STRING_ARRAY
)
