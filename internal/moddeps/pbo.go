// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package moddeps

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

// Entry is one file entry in a PBO's header table.
type Entry struct {
	Name          string
	PackingMethod uint32 // 0 = uncompressed, 0x43707273 ("Cprs") = LZSS-compressed
	OriginalSize  uint32
	DataSize      uint32 // size as stored (== OriginalSize when uncompressed)
	dataOffset    int64
}

const packingCompressed = 0x43707273 // "Cprs" read as a little-endian uint32

// ErrCompressedEntry is returned by ReadEntry for a compressed
// (PackingMethod == "Cprs", LZSS) entry: decompressing it is not
// implemented (see the package doc comment - no real compressed-PBO
// fixture is available here to verify a decoder against).
var ErrCompressedEntry = fmt.Errorf("moddeps: compressed PBO entries are not supported")

// PBO reads a Bohemia PBO archive's header table lazily; call
// ReadEntry to fetch one entry's bytes.
type PBO struct {
	Entries []Entry
	r       io.ReaderAt
}

// OpenPBO parses data's PBO header table (the "Vers" product-info entry,
// if present, is skipped) and returns a PBO ready for ReadEntry calls.
// data must remain valid for the lifetime of the returned PBO (ReadEntry
// reads from it lazily rather than eagerly decompressing every entry).
func OpenPBO(data []byte) (*PBO, error) {
	r := bytes.NewReader(data)
	var entries []Entry
	offset := int64(0)

	readUint32 := func() (uint32, error) {
		var v uint32
		if err := binary.Read(r, binary.LittleEndian, &v); err != nil {
			return 0, err
		}
		offset += 4
		return v, nil
	}
	readCString := func() (string, error) {
		var b bytes.Buffer
		for {
			c, err := r.ReadByte()
			if err != nil {
				return "", err
			}
			offset++
			if c == 0 {
				return b.String(), nil
			}
			b.WriteByte(c)
		}
	}

	for {
		name, err := readCString()
		if err != nil {
			return nil, fmt.Errorf("moddeps: read entry name: %w", err)
		}
		packing, err := readUint32()
		if err != nil {
			return nil, fmt.Errorf("moddeps: read packing method: %w", err)
		}
		original, err := readUint32()
		if err != nil {
			return nil, fmt.Errorf("moddeps: read original size: %w", err)
		}
		if _, err := readUint32(); err != nil { // reserved
			return nil, fmt.Errorf("moddeps: read reserved field: %w", err)
		}
		if _, err := readUint32(); err != nil { // timestamp
			return nil, fmt.Errorf("moddeps: read timestamp: %w", err)
		}
		dataSize, err := readUint32()
		if err != nil {
			return nil, fmt.Errorf("moddeps: read data size: %w", err)
		}

		if name == "" && packing == 0 && original == 0 && dataSize == 0 {
			// The header table's terminating entry.
			break
		}
		if name == "" && packing != 0 {
			// The optional "Vers"/product-info entry (name == "", a
			// non-zero packing "signature" instead): its payload is a
			// sequence of NUL-terminated property/value string pairs
			// ending with an empty string, not a real data blob at
			// dataSize bytes - skip it by reading pairs until empty.
			for {
				k, err := readCString()
				if err != nil {
					return nil, fmt.Errorf("moddeps: read product-info entry: %w", err)
				}
				if k == "" {
					break
				}
				if _, err := readCString(); err != nil {
					return nil, fmt.Errorf("moddeps: read product-info entry: %w", err)
				}
			}
			continue
		}
		entries = append(entries, Entry{Name: name, PackingMethod: packing, OriginalSize: original, DataSize: dataSize})
	}

	// The header table ends at offset; each entry's data follows in
	// order, at cumulative offsets from there.
	dataStart := offset
	for i := range entries {
		entries[i].dataOffset = dataStart
		dataStart += int64(entries[i].DataSize)
	}

	return &PBO{Entries: entries, r: bytes.NewReader(data)}, nil
}

// Find returns the entry named name (case-sensitive, matching PBO's own
// on-disk convention), or false if it is not present.
func (p *PBO) Find(name string) (Entry, bool) {
	for _, e := range p.Entries {
		if e.Name == name {
			return e, true
		}
	}
	return Entry{}, false
}

// ReadEntry returns e's raw bytes. It returns ErrCompressedEntry for a
// compressed entry rather than attempting LZSS decompression.
func (p *PBO) ReadEntry(e Entry) ([]byte, error) {
	if e.PackingMethod == packingCompressed {
		return nil, ErrCompressedEntry
	}
	buf := make([]byte, e.DataSize)
	if _, err := p.r.ReadAt(buf, e.dataOffset); err != nil {
		return nil, fmt.Errorf("moddeps: read entry %q: %w", e.Name, err)
	}
	return buf, nil
}
