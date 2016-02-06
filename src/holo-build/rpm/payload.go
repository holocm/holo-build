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
	"os"
	"os/exec"
	"time"

	"../common"
)

//Payload represents the compressed CPIO payload of the package.
type Payload struct {
	Binary           []byte
	CompressedSize   uint32
	UncompressedSize uint32
}

type cpioHeader struct {
	Magic            [6]byte
	InodeNumber      [8]byte
	Mode             [8]byte
	UID              [8]byte
	GID              [8]byte
	NumberOfLinks    [8]byte
	ModificationTime [8]byte
	FileSize         [8]byte
	DevMajor         [8]byte
	DevMinor         [8]byte
	RdevMajor        [8]byte
	RdevMinor        [8]byte
	NameSize         [8]byte
	Checksum         [8]byte
}

//MakePayload generates the Payload for the given package.
func MakePayload(pkg *common.Package, buildReproducibly bool) (*Payload, error) {
	var timestamp uint32
	if buildReproducibly {
		timestamp = 0
	} else {
		timestamp = uint32(time.Now().Unix())
	}

	var buf bytes.Buffer
	inodeNumber := uint32(0)

	//some fixed values that we can reuse
	cpioOne := cpioFormatInt(1)
	cpioZero := cpioFormatInt(0)
	cpioMagic := [6]byte{'0', '7', '0', '7', '0', '1'}

	//assemble the CPIO archive
	//(NOTE: This traversal works in the same way as the one in addFileInformationTags.)
	pkg.WalkFSWithAbsolutePaths(func(path string, node common.FSNode) error {
		//skip implicitly created directories (as rpmbuild-constructed CPIO
		//archives apparently do)
		if n, ok := node.(*common.FSDirectory); ok {
			if n.Implicit {
				return nil
			}
		}

		inodeNumber++                            //make up inode numbers in the same way as rpmbuild does
		name := append([]byte("."+path), '\000') //must be NUL-terminated!

		header := cpioHeader{
			Magic:       cpioMagic,
			InodeNumber: cpioFormatInt(inodeNumber),
			Mode:        cpioFormatInt(node.FileModeForArchive(true)),
			//UID, GID depend on the node type; see below
			NumberOfLinks:    cpioOne,
			ModificationTime: cpioFormatInt(timestamp),
			//FileSize depends on the node type; see below
			DevMajor:  cpioZero,
			DevMinor:  cpioZero,
			RdevMajor: cpioZero,
			RdevMinor: cpioZero,
			NameSize:  cpioFormatInt(uint32(len(name))),
			Checksum:  cpioZero,
		}

		var data []byte

		switch n := node.(type) {
		case *common.FSDirectory:
			header.UID = cpioFormatInt(n.Metadata.UID())
			header.GID = cpioFormatInt(n.Metadata.GID())
			header.FileSize = cpioZero
		case *common.FSRegularFile:
			header.UID = cpioFormatInt(n.Metadata.UID())
			header.GID = cpioFormatInt(n.Metadata.GID())
			data = []byte(n.Content)
		case *common.FSSymlink:
			header.UID = cpioZero
			header.GID = cpioZero
			data = []byte(n.Target)
		}
		header.FileSize = cpioFormatInt(uint32(len(data)))
		binary.Write(&buf, binary.BigEndian, &header)
		cpioWriteData(&buf, name)
		cpioWriteData(&buf, data)

		return nil
	})

	//write trailer record to indicate the end of the CPIO archive
	trailerName := []byte("TRAILER!!!\000")
	binary.Write(&buf, binary.BigEndian, &cpioHeader{
		Magic:            cpioMagic,
		InodeNumber:      cpioZero,
		Mode:             cpioZero,
		UID:              cpioZero,
		GID:              cpioZero,
		NumberOfLinks:    cpioOne,
		ModificationTime: cpioZero,
		FileSize:         cpioZero,
		DevMajor:         cpioZero,
		DevMinor:         cpioZero,
		RdevMajor:        cpioZero,
		RdevMinor:        cpioZero,
		NameSize:         cpioFormatInt(uint32(len(trailerName))),
		Checksum:         cpioZero,
	})
	cpioWriteData(&buf, trailerName)

	//compress the archive with LZMA
	uncompressed := buf.Bytes()
	cmd := exec.Command("xz", "--format=lzma", "--compress")
	cmd.Stdin = bytes.NewReader(uncompressed)
	cmd.Stderr = os.Stderr
	compressed, err := cmd.Output()

	return &Payload{
		Binary:           compressed,
		CompressedSize:   uint32(len(compressed)),
		UncompressedSize: uint32(len(uncompressed)),
	}, err
}

var hexDigits = []byte("0123456789ABCDEF")

func cpioFormatInt(value uint32) [8]byte {
	var str [8]byte
	for idx := 7; idx >= 0; idx-- {
		str[idx] = hexDigits[value&0xF]
		value = value >> 4
	}
	return str
}

func cpioWriteData(buf *bytes.Buffer, data []byte) {
	buf.Write(data)
	//file names, contents, link targets need to end with padding to 4-byte
	//alignment (note that we cannot compute the padding size from len(data)
	//since the stream is not necessarily 4-byte-aligned before data)
	for buf.Len()%4 != 0 {
		buf.Write([]byte{'\000'})
	}
}
