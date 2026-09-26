// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package servercfg parses and writes DayZ's serverDZ.cfg format: a flat
// list of "key = value;" and "key[] = {v1, v2, ...};" statements, where
// values are either double-quoted strings or bare tokens (numbers). It
// backs FR-12 (serverDZ.cfg managed per instance, with a diff before
// adoption).
package servercfg

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Item is one value inside a scalar or an array entry.
type Item struct {
	Value  string // unquoted text
	Quoted bool
}

func (i Item) render() string {
	if i.Quoted {
		return `"` + strings.ReplaceAll(i.Value, `"`, `""`) + `"`
	}
	return i.Value
}

// Entry is one parsed "key = ...;" statement.
type Entry struct {
	Key     string
	IsArray bool
	Scalar  Item   // valid when !IsArray
	Array   []Item // valid when IsArray
}

// Render returns the value portion of the entry, formatted the way it would
// be written back out. Used for diffing and for the on-disk representation.
func (e *Entry) Render() string {
	if !e.IsArray {
		return e.Scalar.render()
	}
	parts := make([]string, len(e.Array))
	for i, it := range e.Array {
		parts[i] = it.render()
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func (e *Entry) String() string {
	name := e.Key
	if e.IsArray {
		name += "[]"
	}
	return fmt.Sprintf("%s = %s;", name, e.Render())
}

// File is a parsed serverDZ.cfg document: an ordered list of entries.
type File struct {
	entries []*Entry
	index   map[string]int
}

func newFile() *File {
	return &File{index: map[string]int{}}
}

// Keys returns every key in the file, in the order they first appeared.
func (f *File) Keys() []string {
	keys := make([]string, len(f.entries))
	for i, e := range f.entries {
		keys[i] = e.Key
	}
	return keys
}

// Get returns the entry for key, if present.
func (f *File) Get(key string) (*Entry, bool) {
	i, ok := f.index[key]
	if !ok {
		return nil, false
	}
	return f.entries[i], true
}

// SetScalar creates or replaces a scalar entry, keeping its original
// position if it already existed, or appending it otherwise.
func (f *File) SetScalar(key string, value string, quoted bool) {
	e := &Entry{Key: key, Scalar: Item{Value: value, Quoted: quoted}}
	f.set(key, e)
}

// SetArray creates or replaces an array entry.
func (f *File) SetArray(key string, items []Item) {
	e := &Entry{Key: key, IsArray: true, Array: items}
	f.set(key, e)
}

func (f *File) set(key string, e *Entry) {
	if i, ok := f.index[key]; ok {
		f.entries[i] = e
		return
	}
	f.index[key] = len(f.entries)
	f.entries = append(f.entries, e)
}

// Bytes serializes the file, one "key = value;" statement per line, in
// insertion order.
func (f *File) Bytes() []byte {
	var b strings.Builder
	for _, e := range f.entries {
		b.WriteString(e.String())
		b.WriteByte('\n')
	}
	return []byte(b.String())
}

func (f *File) String() string { return string(f.Bytes()) }

// Parse reads a serverDZ.cfg document. It supports "//" and "/* */"
// comments (discarded - a diff is computed on effective values, not on
// formatting) and both scalar and array statements.
func Parse(data []byte) (*File, error) {
	p := &parser{src: string(data)}
	f := newFile()
	for {
		p.skipSpaceAndComments()
		if p.eof() {
			break
		}
		e, err := p.parseEntry()
		if err != nil {
			return nil, fmt.Errorf("servercfg: %w", err)
		}
		f.set(e.Key, e)
	}
	return f, nil
}

type parser struct {
	src string
	pos int
}

func (p *parser) eof() bool { return p.pos >= len(p.src) }

func (p *parser) skipSpaceAndComments() {
	for !p.eof() {
		c := p.src[p.pos]
		switch {
		case c == ' ' || c == '\t' || c == '\r' || c == '\n':
			p.pos++
		case c == '/' && p.peek(1) == '/':
			for !p.eof() && p.src[p.pos] != '\n' {
				p.pos++
			}
		case c == '/' && p.peek(1) == '*':
			end := strings.Index(p.src[p.pos+2:], "*/")
			if end < 0 {
				p.pos = len(p.src)
				return
			}
			p.pos += 2 + end + 2
		default:
			return
		}
	}
}

func (p *parser) peek(offset int) byte {
	if p.pos+offset >= len(p.src) {
		return 0
	}
	return p.src[p.pos+offset]
}

func isKeyChar(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

func (p *parser) parseEntry() (*Entry, error) {
	start := p.pos
	for !p.eof() && isKeyChar(p.src[p.pos]) {
		p.pos++
	}
	key := p.src[start:p.pos]
	if key == "" {
		return nil, fmt.Errorf("unexpected character %q at offset %d", p.src[p.pos], p.pos)
	}

	p.skipSpaceAndComments()
	isArray := false
	if p.peek(0) == '[' && p.peek(1) == ']' {
		isArray = true
		p.pos += 2
		p.skipSpaceAndComments()
	}
	if p.eof() || p.src[p.pos] != '=' {
		return nil, fmt.Errorf("expected '=' after key %q", key)
	}
	p.pos++
	p.skipSpaceAndComments()

	e := &Entry{Key: key, IsArray: isArray}
	if isArray {
		items, err := p.parseArray()
		if err != nil {
			return nil, err
		}
		e.Array = items
	} else {
		item, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		e.Scalar = item
	}

	p.skipSpaceAndComments()
	if p.eof() || p.src[p.pos] != ';' {
		return nil, fmt.Errorf("expected ';' after value of key %q", key)
	}
	p.pos++
	return e, nil
}

func (p *parser) parseArray() ([]Item, error) {
	if p.eof() || p.src[p.pos] != '{' {
		return nil, fmt.Errorf("expected '{' to start array")
	}
	p.pos++
	var items []Item
	for {
		p.skipSpaceAndComments()
		if p.eof() {
			return nil, fmt.Errorf("unterminated array")
		}
		if p.src[p.pos] == '}' {
			p.pos++
			return items, nil
		}
		item, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		items = append(items, item)
		p.skipSpaceAndComments()
		if !p.eof() && p.src[p.pos] == ',' {
			p.pos++
			continue
		}
	}
}

func (p *parser) parseValue() (Item, error) {
	if p.eof() {
		return Item{}, fmt.Errorf("unexpected end of input while reading a value")
	}
	if p.src[p.pos] == '"' {
		p.pos++
		var sb strings.Builder
		for {
			if p.eof() {
				return Item{}, fmt.Errorf("unterminated string literal")
			}
			c := p.src[p.pos]
			if c == '"' {
				// DayZ/BIS config strings escape an embedded quote as "".
				if p.peek(1) == '"' {
					sb.WriteByte('"')
					p.pos += 2
					continue
				}
				p.pos++
				return Item{Value: sb.String(), Quoted: true}, nil
			}
			sb.WriteByte(c)
			p.pos++
		}
	}

	start := p.pos
	for !p.eof() && !strings.ContainsRune(",;}", rune(p.src[p.pos])) {
		p.pos++
	}
	raw := strings.TrimSpace(p.src[start:p.pos])
	if raw == "" {
		return Item{}, fmt.Errorf("empty bare value at offset %d", start)
	}
	return Item{Value: raw, Quoted: false}, nil
}

// ChangedEntry describes a key whose rendered value differs between two files.
type ChangedEntry struct {
	Key      string
	Old, New string
}

// Diff is the result of comparing two serverDZ.cfg files by key (FR-12).
type Diff struct {
	Added   []string
	Removed []string
	Changed []ChangedEntry
}

// Empty reports whether the two files are equivalent.
func (d Diff) Empty() bool {
	return len(d.Added) == 0 && len(d.Removed) == 0 && len(d.Changed) == 0
}

// DiffFiles compares oldFile (e.g. the currently adopted config) against
// newFile (e.g. the one rendered from the site repo), by key, ignoring
// formatting and key order.
func DiffFiles(oldFile, newFile *File) Diff {
	var d Diff
	for _, key := range newFile.Keys() {
		newEntry, _ := newFile.Get(key)
		oldEntry, existed := oldFile.Get(key)
		if !existed {
			d.Added = append(d.Added, key)
			continue
		}
		if oldEntry.Render() != newEntry.Render() {
			d.Changed = append(d.Changed, ChangedEntry{
				Key: key,
				Old: oldEntry.Render(),
				New: newEntry.Render(),
			})
		}
	}
	for _, key := range oldFile.Keys() {
		if _, ok := newFile.Get(key); !ok {
			d.Removed = append(d.Removed, key)
		}
	}
	sort.Strings(d.Added)
	sort.Strings(d.Removed)
	sort.Slice(d.Changed, func(i, j int) bool { return d.Changed[i].Key < d.Changed[j].Key })
	return d
}

// AsInt parses a scalar entry's value as an integer, for numeric keys such
// as maxPlayers or steamQueryPort.
func (e *Entry) AsInt() (int, error) {
	if e.IsArray {
		return 0, fmt.Errorf("servercfg: %s is an array, not a scalar", e.Key)
	}
	return strconv.Atoi(e.Scalar.Value)
}
