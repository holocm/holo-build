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
	"crypto/md5"
	"crypto/sha1"
	"encoding/hex"
)

//makeSignatureSection produces the signature section of an RPM header.
func makeSignatureSection(headerSection []byte, payload *rpmPayload) []byte {
	h := &rpmHeader{}

	//NOTE that some fields validate both header+payload, some only the
	//payload, and some only the header. This is all according to the
	//specification, no matter how insane. [LSB, 25.2.3]

	//size information
	h.AddInt32Value(rpmsigtagSize, []int32{
		int32(uint32(len(headerSection)) + payload.CompressedSize),
	})
	h.AddInt32Value(rpmsigtagPayloadSize, []int32{
		int32(payload.UncompressedSize),
	})

	//SHA1 digest of header section
	sha1digest := sha1.New()
	sha1digest.Write(headerSection)
	sha1sum := hex.EncodeToString(sha1digest.Sum(nil))
	h.AddStringValue(rpmsigtagSHA1, sha1sum, false)

	//MD5 digest of header + payload section
	md5digest := md5.New()
	md5digest.Write(headerSection)
	md5digest.Write(payload.Binary)
	md5sum := md5digest.Sum(nil)
	h.AddBinaryValue(rpmsigtagMD5, md5sum)

	return h.ToBinary(rpmtagHeaderSignatures)
}
