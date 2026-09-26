// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package servercfg

import (
	"strings"
	"testing"
)

const sample = `
// this is the hostname
hostname = "dzo test server";
maxPlayers = 60;
verifySignatures = 2;
disableVoN = 0;
beLoggerFilters[] = {"KillHacker", "TeleportHacker"};
motd[] = {"Welcome", "Have fun"};
/* block comment
   spanning lines */
template = "dayzOffline.chernarusplus";
`

func TestParseBasic(t *testing.T) {
	f, err := Parse([]byte(sample))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	wantKeys := []string{"hostname", "maxPlayers", "verifySignatures", "disableVoN", "beLoggerFilters", "motd", "template"}
	if got := f.Keys(); !equalStrings(got, wantKeys) {
		t.Fatalf("Keys() = %v, want %v", got, wantKeys)
	}

	hostname, ok := f.Get("hostname")
	if !ok {
		t.Fatal("hostname not found")
	}
	if hostname.Scalar.Value != "dzo test server" || !hostname.Scalar.Quoted {
		t.Errorf("hostname = %+v", hostname.Scalar)
	}

	maxPlayers, ok := f.Get("maxPlayers")
	if !ok {
		t.Fatal("maxPlayers not found")
	}
	n, err := maxPlayers.AsInt()
	if err != nil || n != 60 {
		t.Errorf("maxPlayers.AsInt() = %d, %v, want 60, nil", n, err)
	}

	motd, ok := f.Get("motd")
	if !ok || !motd.IsArray || len(motd.Array) != 2 {
		t.Fatalf("motd = %+v", motd)
	}
	if motd.Array[0].Value != "Welcome" || motd.Array[1].Value != "Have fun" {
		t.Errorf("motd array = %+v", motd.Array)
	}
}

func TestAsIntOnArrayFails(t *testing.T) {
	f, err := Parse([]byte(sample))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	motd, _ := f.Get("motd")
	if _, err := motd.AsInt(); err == nil {
		t.Fatal("expected an error calling AsInt on an array entry")
	}
}

func TestRoundTrip(t *testing.T) {
	f, err := Parse([]byte(sample))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	out := f.String()

	f2, err := Parse([]byte(out))
	if err != nil {
		t.Fatalf("re-parse of rendered output: %v\noutput was:\n%s", err, out)
	}
	if !equalStrings(f.Keys(), f2.Keys()) {
		t.Fatalf("keys changed across round trip: %v vs %v", f.Keys(), f2.Keys())
	}
	for _, k := range f.Keys() {
		e1, _ := f.Get(k)
		e2, _ := f2.Get(k)
		if e1.Render() != e2.Render() {
			t.Errorf("key %s: %q != %q after round trip", k, e1.Render(), e2.Render())
		}
	}
}

func TestSetScalarUpdatesInPlace(t *testing.T) {
	f, err := Parse([]byte(sample))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	before := f.Keys()
	f.SetScalar("maxPlayers", "100", false)
	after := f.Keys()
	if !equalStrings(before, after) {
		t.Fatalf("SetScalar on an existing key should not change key order: %v vs %v", before, after)
	}
	e, _ := f.Get("maxPlayers")
	if e.Render() != "100" {
		t.Errorf("maxPlayers = %s, want 100", e.Render())
	}
}

func TestSetScalarAppendsNewKey(t *testing.T) {
	f, err := Parse([]byte(sample))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	f.SetScalar("steamQueryPort", "27016", false)
	keys := f.Keys()
	if keys[len(keys)-1] != "steamQueryPort" {
		t.Fatalf("new key should be appended, got keys: %v", keys)
	}
}

func TestSetArray(t *testing.T) {
	f := newFile()
	f.SetArray("motd", []Item{{Value: "a", Quoted: true}, {Value: "b", Quoted: true}})
	e, ok := f.Get("motd")
	if !ok || !e.IsArray {
		t.Fatalf("motd = %+v", e)
	}
	if e.Render() != `{"a", "b"}` {
		t.Errorf("Render() = %q", e.Render())
	}
}

func TestDiffFiles(t *testing.T) {
	oldFile, err := Parse([]byte(`hostname = "old"; maxPlayers = 40; disableVoN = 0;`))
	if err != nil {
		t.Fatalf("Parse old: %v", err)
	}
	newFile, err := Parse([]byte(`hostname = "new"; maxPlayers = 40; steamQueryPort = 27016;`))
	if err != nil {
		t.Fatalf("Parse new: %v", err)
	}

	d := DiffFiles(oldFile, newFile)
	if d.Empty() {
		t.Fatal("expected a non-empty diff")
	}
	if !equalStrings(d.Added, []string{"steamQueryPort"}) {
		t.Errorf("Added = %v", d.Added)
	}
	if !equalStrings(d.Removed, []string{"disableVoN"}) {
		t.Errorf("Removed = %v", d.Removed)
	}
	if len(d.Changed) != 1 || d.Changed[0].Key != "hostname" {
		t.Fatalf("Changed = %+v", d.Changed)
	}
	if d.Changed[0].Old != `"old"` || d.Changed[0].New != `"new"` {
		t.Errorf("Changed[0] = %+v", d.Changed[0])
	}
}

func TestDiffFilesEmptyWhenEqual(t *testing.T) {
	a, err := Parse([]byte(`hostname = "same";`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	b, err := Parse([]byte(`hostname = "same";`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if d := DiffFiles(a, b); !d.Empty() {
		t.Fatalf("expected an empty diff, got %+v", d)
	}
}

func TestParseErrors(t *testing.T) {
	cases := []string{
		`= "no key";`,
		`hostname "missing equals";`,
		`hostname = "unterminated`,
		`beLoggerFilters[] = {"a", "b"`,
		`beLoggerFilters[] = "not an array";`,
		`hostname = ;`,
	}
	for _, c := range cases {
		if _, err := Parse([]byte(c)); err == nil {
			t.Errorf("Parse(%q) should have failed", c)
		}
	}
}

func TestParseEmpty(t *testing.T) {
	f, err := Parse([]byte("  \n// just a comment\n/* and a block */\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(f.Keys()) != 0 {
		t.Fatalf("expected no keys, got %v", f.Keys())
	}
}

func TestEntryStringFormatsArrayKey(t *testing.T) {
	f := newFile()
	f.SetArray("motd", []Item{{Value: "hi", Quoted: true}})
	e, _ := f.Get("motd")
	if !strings.HasPrefix(e.String(), "motd[] = ") {
		t.Errorf("String() = %q", e.String())
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
