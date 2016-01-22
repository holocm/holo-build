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

package impl

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
)

//DumpRpm dumps RPM packages.
func DumpRpm(data []byte) (string, error) {
	//We don't have a library for the RPM format, and unfortunately, it's an utter mess.
	//The main reference that I used (apart from sample RPMs from Fedora, Mageia, and Suse)
	//is <http://www.rpm.org/max-rpm/s1-rpm-file-format-rpm-file-format.html> and
	//<https://docs.fedoraproject.org/ro/Fedora_Draft_Documentation/0.1/html/RPM_Guide/ch-package-structure.html>.
	reader := bytes.NewReader(data)

	//decode the various header structures
	leadDump, err := dumpRpmLead(reader)
	if err != nil {
		return "", err
	}
	signatureDump, err := dumpRpmHeader(reader, "signature", true, rpmtagDictForSignatureHeader)
	if err != nil {
		return "", err
	}
	headerDump, err := dumpRpmHeader(reader, "header", false, rpmtagDictForMetadataHeader)
	if err != nil {
		return "", err
	}

	//decode payload
	payloadData, err := ioutil.ReadAll(reader)
	if err != nil {
		return "", err
	}
	payloadDump, err := RecognizeAndDump(payloadData)
	if err != nil {
		return "", err
	}

	return "RPM package\n" + Indent(leadDump) + Indent(signatureDump) + Indent(headerDump) + Indent(">> payload: "+payloadDump), nil
}

func dumpRpmLead(reader io.Reader) (string, error) {
	//read the lead (the initial fixed-size header)
	var lead struct {
		Magic         uint32
		MajorVersion  uint8
		MinorVersion  uint8
		Type          uint16
		Architecture  uint16
		Name          [66]byte
		OSNum         uint16
		SignatureType uint16
		Reserved      [16]byte
	}
	err := binary.Read(reader, binary.BigEndian, &lead)
	if err != nil {
		return "", err
	}

	lines := []string{
		fmt.Sprintf("RPM format version %d.%d", lead.MajorVersion, lead.MinorVersion),
		fmt.Sprintf("Type: %d (0 = binary, 1 = source)", lead.Type),
		fmt.Sprintf("Architecture: %d (0 = noarch, 1 = x86, ...)", lead.Architecture),
		//lead.Name is a NUL-terminated (and NUL-padded) string; trim all the NULs at the end
		fmt.Sprintf("Name: %s", strings.TrimRight(string(lead.Name[:]), "\x00")),
		fmt.Sprintf("Built for OS: %d (1 = Linux, ...)", lead.OSNum),
		fmt.Sprintf("Signature type: %d", lead.SignatureType),
	}
	return ">> lead section:\n" + Indent(strings.Join(lines, "\n")), nil
}

//IndexEntry represents an entry in the index of an RPM header.
type IndexEntry struct {
	Tag    uint32 //defines the semantics of the value in this field
	Type   uint32 //data type
	Offset uint32 //relative to the beginning of the store
	Count  uint32 //number of data items in this field
}

func dumpRpmHeader(reader io.Reader, sectionIdent string, readAligned bool, tagDict map[uint32]string) (string, error) {
	//the header has a header (I'm So Meta, Even This Acronym)
	var header struct {
		Magic      [3]byte
		Version    uint8
		Reserved   [4]byte
		EntryCount uint32 //supports 4 billion header entries... Now that's planning ahead! :)
		DataSize   uint32 //size of the store (i.e. the data section, everything after the index until the end of the header section)
	}
	err := binary.Read(reader, binary.BigEndian, &header)
	if err != nil {
		return "", err
	}
	if header.Magic != [3]byte{0x8e, 0xad, 0xe8} {
		return "", fmt.Errorf(
			"did not find RPM header structure header at expected position (saw 0x%s instead of 0x8eade8)",
			hex.EncodeToString(header.Magic[:]),
		)
	}
	identifier := fmt.Sprintf(">> %s section: format version %d, %d entries, %d bytes of data\n",
		sectionIdent, header.Version, header.EntryCount, header.DataSize,
	)

	//read index of fields
	indexEntries := make([]IndexEntry, 0, header.EntryCount)
	for idx := uint32(0); idx < header.EntryCount; idx++ {
		var entry IndexEntry
		err := binary.Read(reader, binary.BigEndian, &entry)
		if err != nil {
			return "", err
		}
		indexEntries = append(indexEntries, entry)
	}

	//read remaining part of header (the data store) into a buffer for random access
	buffer := make([]byte, header.DataSize)
	_, err = io.ReadFull(reader, buffer)
	if err != nil {
		return "", err
	}
	bufferedReader := bytes.NewReader(buffer)

	if readAligned {
		//next structure in reader is aligned to 8-byte boundary -- skip over padding
		modulo := header.DataSize % 8
		if modulo != 0 {
			_, err = io.ReadFull(reader, make([]byte, 8-modulo))
			if err != nil {
				return "", err
			}
		}
	}

	//decode and dump entries
	lines := make([]string, 0, len(indexEntries)) //lower estimate only
	for _, entry := range indexEntries {
		//seek to start of entry
		_, err := bufferedReader.Seek(int64(entry.Offset), 0)
		if err != nil {
			return "", err
		}

		var sublines []string
		if entry.Type == 7 {
			//for entry.Type = 7 (BIN), entry.Count is the number of bytes to be read
			data := make([]byte, entry.Count)
			_, err = io.ReadFull(bufferedReader, data)
			if err != nil {
				return "", err
			}
			sublines = []string{hex.Dump(data)}
		} else {
			//for all other types, entry.Count tells the number of records to read
			sublines = make([]string, 0, entry.Count)
			for idx := uint32(0); idx < entry.Count; idx++ {
				repr, err := decodeIndexEntry(entry.Type, bufferedReader)
				if err != nil {
					return "", err
				}
				sublines = append(sublines, repr)
			}
		}

		//identify entry by looking up the tag name
		tagName, isKnownTag := tagDict[entry.Tag]
		if isKnownTag {
			tagName = fmt.Sprintf("tag %d (%s)", entry.Tag, tagName)
		} else {
			tagName = fmt.Sprintf("tag %d", entry.Tag)
		}

		line := fmt.Sprintf("%s: length %d\n", tagName, entry.Count)
		lines = append(lines, line+strings.TrimSuffix(Indent(strings.Join(sublines, "\n")), "\n"))
	}

	return identifier + Indent(strings.Join(lines, "\n")), nil
}

func decodeIndexEntry(dataType uint32, reader io.Reader) (string, error) {
	//check data type
	switch dataType {
	case 0: //NULL
		return "null", nil
	case 1: //CHAR
		var value uint8
		err := binary.Read(reader, binary.BigEndian, &value)
		return fmt.Sprintf("char: %c", rune(value)), err
	case 2: //INT8
		var value int8
		err := binary.Read(reader, binary.BigEndian, &value)
		return fmt.Sprintf("int8: %d", value), err
	case 3: //INT16
		var value int16
		err := binary.Read(reader, binary.BigEndian, &value)
		return fmt.Sprintf("int16: %d", value), err
	case 4: //INT32
		var value int32
		err := binary.Read(reader, binary.BigEndian, &value)
		return fmt.Sprintf("int32: %d", value), err
	case 5: //INT64
		var value int64
		err := binary.Read(reader, binary.BigEndian, &value)
		return fmt.Sprintf("int64: %d", value), err
	case 7: //BIN
		panic("Cannot be reached")
	case 6, 8: //STRING or STRING_ARRAY or I18NSTRING (not different at this point)
		str, err := readNulTerminatedString(reader)
		return fmt.Sprintf("string: %s", str), err
	case 9: //I18N_STRING
		str, err := readNulTerminatedString(reader)
		return fmt.Sprintf("translatable string: %s", str), err
	default:
		return fmt.Sprintf("don't know how to decode data type %d", dataType), nil
	}
}

func readNulTerminatedString(reader io.Reader) (string, error) {
	//read NUL-terminated string (byte-wise)
	var result []byte
	buffer := make([]byte, 1)
	for {
		_, err := reader.Read(buffer)
		if err != nil {
			return "", err
		}
		if buffer[0] == 0 {
			break
		} else {
			result = append(result, buffer[0])
		}
	}
	return string(result), nil
}

////////////////////////////////////////////////////////////////////////////////
// mappings of tag ID -> tag name (as extracted from /usr/include/rpm/rpmtag.h)

var rpmtagDictForSignatureHeader = map[uint32]string{
	62:   "HEADERSIGNATURES",
	1000: "SIZE",
	1001: "LEMD5_1",
	1002: "PGP",
	1003: "LEMD5_2",
	1004: "MD5",
	1005: "GPG",
	1006: "PGP5",
	1007: "PAYLOADSIZE",
	1008: "RESERVEDSPACE",
	264:  "BADSHA1_1",
	265:  "BADSHA1_2",
	267:  "DSA",
	268:  "RSA",
	269:  "SHA1",
	270:  "LONGSIZE",
	271:  "LONGARCHIVESIZE",
}

var rpmtagDictForMetadataHeader = map[uint32]string{
	63:   "HEADERIMMUTABLE",
	100:  "HEADERI18NTABLE",
	1000: "NAME",
	1001: "VERSION",
	1002: "RELEASE",
	1003: "EPOCH",
	1004: "SUMMARY",
	1005: "DESCRIPTION",
	1006: "BUILDTIME",
	1007: "BUILDHOST",
	1008: "INSTALLTIME",
	1009: "SIZE",
	1010: "DISTRIBUTION",
	1011: "VENDOR",
	1012: "GIF",
	1013: "XPM",
	1014: "LICENSE",
	1015: "PACKAGER",
	1016: "GROUP",
	1017: "CHANGELOG",
	1018: "SOURCE",
	1019: "PATCH",
	1020: "URL",
	1021: "OS",
	1022: "ARCH",
	1023: "PREIN",
	1024: "POSTIN",
	1025: "PREUN",
	1026: "POSTUN",
	1027: "OLDFILENAMES",
	1028: "FILESIZES",
	1029: "FILESTATES",
	1030: "FILEMODES",
	1031: "FILEUIDS",
	1032: "FILEGIDS",
	1033: "FILERDEVS",
	1034: "FILEMTIMES",
	1035: "FILEMD5S",
	1036: "FILELINKTOS",
	1037: "FILEFLAGS",
	1038: "ROOT",
	1039: "FILEUSERNAME",
	1040: "FILEGROUPNAME",
	1041: "EXCLUDE",
	1042: "EXCLUSIVE",
	1043: "ICON",
	1044: "SOURCERPM",
	1045: "FILEVERIFYFLAGS",
	1046: "ARCHIVESIZE",
	1047: "PROVIDENAME",
	1048: "REQUIREFLAGS",
	1049: "REQUIRENAME",
	1050: "REQUIREVERSION",
	1051: "NOSOURCE",
	1052: "NOPATCH",
	1053: "CONFLICTFLAGS",
	1054: "CONFLICTNAME",
	1055: "CONFLICTVERSION",
	1056: "DEFAULTPREFIX",
	1057: "BUILDROOT",
	1058: "INSTALLPREFIX",
	1059: "EXCLUDEARCH",
	1060: "EXCLUDEOS",
	1061: "EXCLUSIVEARCH",
	1062: "EXCLUSIVEOS",
	1063: "AUTOREQPROV",
	1064: "RPMVERSION",
	1065: "TRIGGERSCRIPTS",
	1066: "TRIGGERNAME",
	1067: "TRIGGERVERSION",
	1068: "TRIGGERFLAGS",
	1069: "TRIGGERINDEX",
	1079: "VERIFYSCRIPT",
	1080: "CHANGELOGTIME",
	1081: "CHANGELOGNAME",
	1082: "CHANGELOGTEXT",
	1083: "BROKENMD5",
	1084: "PREREQ",
	1085: "PREINPROG",
	1086: "POSTINPROG",
	1087: "PREUNPROG",
	1088: "POSTUNPROG",
	1089: "BUILDARCHS",
	1090: "OBSOLETENAME",
	1091: "VERIFYSCRIPTPROG",
	1092: "TRIGGERSCRIPTPROG",
	1093: "DOCDIR",
	1094: "COOKIE",
	1095: "FILEDEVICES",
	1096: "FILEINODES",
	1097: "FILELANGS",
	1098: "PREFIXES",
	1099: "INSTPREFIXES",
	1100: "TRIGGERIN",
	1101: "TRIGGERUN",
	1102: "TRIGGERPOSTUN",
	1103: "AUTOREQ",
	1104: "AUTOPROV",
	1105: "CAPABILITY",
	1106: "SOURCEPACKAGE",
	1107: "OLDORIGFILENAMES",
	1108: "BUILDPREREQ",
	1109: "BUILDREQUIRES",
	1110: "BUILDCONFLICTS",
	1111: "BUILDMACROS",
	1112: "PROVIDEFLAGS",
	1113: "PROVIDEVERSION",
	1114: "OBSOLETEFLAGS",
	1115: "OBSOLETEVERSION",
	1116: "DIRINDEXES",
	1117: "BASENAMES",
	1118: "DIRNAMES",
	1119: "ORIGDIRINDEXES",
	1120: "ORIGBASENAMES",
	1121: "ORIGDIRNAMES",
	1122: "OPTFLAGS",
	1123: "DISTURL",
	1124: "PAYLOADFORMAT",
	1125: "PAYLOADCOMPRESSOR",
	1126: "PAYLOADFLAGS",
	1127: "INSTALLCOLOR",
	1128: "INSTALLTID",
	1129: "REMOVETID",
	1130: "SHA1RHN",
	1131: "RHNPLATFORM",
	1132: "PLATFORM",
	1133: "PATCHESNAME",
	1134: "PATCHESFLAGS",
	1135: "PATCHESVERSION",
	1136: "CACHECTIME",
	1137: "CACHEPKGPATH",
	1138: "CACHEPKGSIZE",
	1139: "CACHEPKGMTIME",
	1140: "FILECOLORS",
	1141: "FILECLASS",
	1142: "CLASSDICT",
	1143: "FILEDEPENDSX",
	1144: "FILEDEPENDSN",
	1145: "DEPENDSDICT",
	1146: "SOURCEPKGID",
	1147: "FILECONTEXTS",
	1148: "FSCONTEXTS",
	1149: "RECONTEXTS",
	1150: "POLICIES",
	1151: "PRETRANS",
	1152: "POSTTRANS",
	1153: "PRETRANSPROG",
	1154: "POSTTRANSPROG",
	1155: "DISTTAG",
	1156: "OLDSUGGESTSNAME",
	1157: "OLDSUGGESTSVERSION",
	1158: "OLDSUGGESTSFLAGS",
	1159: "OLDENHANCESNAME",
	1160: "OLDENHANCESVERSION",
	1161: "OLDENHANCESFLAGS",
	1162: "PRIORITY",
	1163: "CVSID",
	1164: "BLINKPKGID",
	1165: "BLINKHDRID",
	1166: "BLINKNEVRA",
	1167: "FLINKPKGID",
	1168: "FLINKHDRID",
	1169: "FLINKNEVRA",
	1170: "PACKAGEORIGIN",
	1171: "TRIGGERPREIN",
	1172: "BUILDSUGGESTS",
	1173: "BUILDENHANCES",
	1174: "SCRIPTSTATES",
	1175: "SCRIPTMETRICS",
	1176: "BUILDCPUCLOCK",
	1177: "FILEDIGESTALGOS",
	1178: "VARIANTS",
	1179: "XMAJOR",
	1180: "XMINOR",
	1181: "REPOTAG",
	1182: "KEYWORDS",
	1183: "BUILDPLATFORMS",
	1184: "PACKAGECOLOR",
	1185: "PACKAGEPREFCOLOR",
	1186: "XATTRSDICT",
	1187: "FILEXATTRSX",
	1188: "DEPATTRSDICT",
	1189: "CONFLICTATTRSX",
	1190: "OBSOLETEATTRSX",
	1191: "PROVIDEATTRSX",
	1192: "REQUIREATTRSX",
	1193: "BUILDPROVIDES",
	1194: "BUILDOBSOLETES",
	1195: "DBINSTANCE",
	1196: "NVRA",
	5000: "FILENAMES",
	5001: "FILEPROVIDE",
	5002: "FILEREQUIRE",
	5003: "FSNAMES",
	5004: "FSSIZES",
	5005: "TRIGGERCONDS",
	5006: "TRIGGERTYPE",
	5007: "ORIGFILENAMES",
	5008: "LONGFILESIZES",
	5009: "LONGSIZE",
	5010: "FILECAPS",
	5011: "FILEDIGESTALGO",
	5012: "BUGURL",
	5013: "EVR",
	5014: "NVR",
	5015: "NEVR",
	5016: "NEVRA",
	5017: "HEADERCOLOR",
	5018: "VERBOSE",
	5019: "EPOCHNUM",
	5020: "PREINFLAGS",
	5021: "POSTINFLAGS",
	5022: "PREUNFLAGS",
	5023: "POSTUNFLAGS",
	5024: "PRETRANSFLAGS",
	5025: "POSTTRANSFLAGS",
	5026: "VERIFYSCRIPTFLAGS",
	5027: "TRIGGERSCRIPTFLAGS",
	5029: "COLLECTIONS",
	5030: "POLICYNAMES",
	5031: "POLICYTYPES",
	5032: "POLICYTYPESINDEXES",
	5033: "POLICYFLAGS",
	5034: "VCS",
	5035: "ORDERNAME",
	5036: "ORDERVERSION",
	5037: "ORDERFLAGS",
	5038: "MSSFMANIFEST",
	5039: "MSSFDOMAIN",
	5040: "INSTFILENAMES",
	5041: "REQUIRENEVRS",
	5042: "PROVIDENEVRS",
	5043: "OBSOLETENEVRS",
	5044: "CONFLICTNEVRS",
	5045: "FILENLINKS",
	5046: "RECOMMENDNAME",
	5047: "RECOMMENDVERSION",
	5048: "RECOMMENDFLAGS",
	5049: "SUGGESTNAME",
	5050: "SUGGESTVERSION",
	5051: "SUGGESTFLAGS",
	5052: "SUPPLEMENTNAME",
	5053: "SUPPLEMENTVERSION",
	5054: "SUPPLEMENTFLAGS",
	5055: "ENHANCENAME",
	5056: "ENHANCEVERSION",
	5057: "ENHANCEFLAGS",
	5058: "RECOMMENDNEVRS",
	5059: "SUGGESTNEVRS",
	5060: "SUPPLEMENTNEVRS",
	5061: "ENHANCENEVRS",
}
