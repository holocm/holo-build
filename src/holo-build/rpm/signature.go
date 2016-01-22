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
	"crypto/md5"
	"crypto/sha1"
	"encoding/hex"
)

//MakeSignatureSection produces the signature section of an RPM header.
func MakeSignatureSection(headerSection []byte, payload *Payload) []byte {
	h := &Header{}

	//NOTE that some fields validate both header+payload, some only the
	//payload, and some only the header. This is all according to the
	//specification, no matter how insane. [LSB, 22.2.3]

	//size information
	h.AddInt32Value(RpmsigtagSize, []int32{
		int32(uint32(len(headerSection)) + payload.CompressedSize),
	})
	h.AddInt32Value(RpmsigtagPayloadSize, []int32{
		int32(payload.UncompressedSize),
	})

	//SHA1 digest of header section
	sha1digest := sha1.New()
	sha1digest.Write(headerSection)
	sha1sum := hex.EncodeToString(sha1digest.Sum(nil))
	h.AddStringValue(RpmsigtagSHA1, sha1sum, false)

	//MD5 digest of header + payload section
	md5digest := md5.New()
	md5digest.Write(headerSection)
	md5digest.Write(payload.Binary)
	md5sum := md5digest.Sum(nil)
	h.AddBinaryValue(RpmsigtagMD5, md5sum)

	return h.ToBinary()
}
