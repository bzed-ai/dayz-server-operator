// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package moddeps

import "errors"

// decompressLZSS decodes a PBO "Cprs" entry into size bytes. Format source:
// the community PBO documentation and the armake/DePbo readers. The stream
// is groups of one flag byte followed by eight items (flag bits read LSB
// first): bit 1 = one literal byte; bit 0 = a two-byte back reference
// b1,b2 with position = b1 | (b2&0xF0)<<4 (an offset into a 4096-byte ring
// indexed by output position) and length = (b2&0x0F)+3. Ring bytes before
// the start of the output read as spaces. A 4-byte additive checksum
// trails the stream; it is ignored. Verified only against a synthetic
// encoder in the tests, not a real mod PBO.
func decompressLZSS(in []byte, size int) ([]byte, error) {
	out := make([]byte, 0, size)
	i := 0
	for len(out) < size {
		if i >= len(in) {
			return nil, errors.New("truncated LZSS data")
		}
		flags := in[i]
		i++
		for bit := 0; bit < 8 && len(out) < size; bit++ {
			if flags&1 != 0 {
				if i >= len(in) {
					return nil, errors.New("truncated LZSS data")
				}
				out = append(out, in[i])
				i++
			} else {
				if i+2 > len(in) {
					return nil, errors.New("truncated LZSS data")
				}
				b1, b2 := int(in[i]), int(in[i+1])
				i += 2
				rpos := b1 | (b2&0xF0)<<4
				n := b2&0x0F + 3
				src := len(out) - ((len(out) - rpos) & 0xFFF)
				for ; n > 0 && len(out) < size; n-- {
					if src < 0 {
						out = append(out, ' ')
					} else {
						out = append(out, out[src])
					}
					src++
				}
			}
			flags >>= 1
		}
	}
	return out, nil
}
