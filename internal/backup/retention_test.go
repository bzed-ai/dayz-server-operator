// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package backup

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
)

var t0 = time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

func snaps(reasons ...Reason) []Snapshot {
	var out []Snapshot
	for i, r := range reasons {
		out = append(out, Snapshot{ID: fmt.Sprintf("s%02d", i), Reason: r, State: Complete, Created: t0.Add(time.Duration(i) * time.Hour)})
	}
	return out
}

func ids(ss []Snapshot) string {
	s := ""
	for _, x := range ss {
		s += x.ID + " "
	}
	return s
}

func TestSelectKeepsTheNewest(t *testing.T) {
	ss := snaps(Manual, Manual, Manual, Manual, Manual)
	got := Select(ss, Policy{Keep: 2, MinKeep: 1}, t0.Add(10*time.Hour))
	if ids(got) != "s00 s01 s02 " {
		t.Errorf("keep 2 of 5 must delete the three oldest, got %s", ids(got))
	}
	if got := Select(ss, Policy{}, t0.Add(time.Hour)); len(got) != 0 {
		t.Errorf("no limits delete nothing: %s", ids(got))
	}
}

func TestSelectPerReasonAndAge(t *testing.T) {
	ss := snaps(ModUpdate, Update, ModUpdate, ModUpdate, Manual, ModUpdate)
	got := Select(ss, Policy{KeepByReason: map[Reason]int{ModUpdate: 2}, MinKeep: 1}, t0.Add(10*time.Hour))
	if ids(got) != "s00 s02 " {
		t.Errorf("keep 2 mod_update (the two newest) must delete s00 and s02, got %s", ids(got))
	}
	got = Select(ss, Policy{MaxAge: 3*time.Hour + time.Minute, MinKeep: 1}, t0.Add(6*time.Hour))
	if ids(got) != "s00 s01 s02 " {
		t.Errorf("older than 3h at t+6h: s00..s02 (s03 is 3h old), got %s", ids(got))
	}
}

func TestSelectProtections(t *testing.T) {
	ss := snaps(Manual, Manual, Manual, Manual)
	ss[0].Pinned = true
	ss[1].State = Creating
	ss[2].State = Deleting
	got := Select(ss, Policy{Keep: 1, MaxAge: time.Minute, MinKeep: 0}, t0.Add(48*time.Hour))
	if len(got) != 0 {
		t.Errorf("pinned, creating and deleting are not candidates and the newest stays: %s", ids(got))
	}

	ss = snaps(Manual, Manual, Manual, Manual, Manual)
	got = Select(ss, Policy{MaxAge: time.Minute, MinKeep: 3}, t0.Add(48*time.Hour))
	if ids(got) != "s00 s01 " {
		t.Errorf("min_keep 3 of 5 old snapshots: delete the two oldest, got %s", ids(got))
	}
	// a pinned snapshot counts toward min_keep
	ss[4].Pinned = true
	got = Select(ss, Policy{MaxAge: time.Minute, MinKeep: 3}, t0.Add(48*time.Hour))
	if ids(got) != "s00 s01 " {
		t.Errorf("pinned counts, got %s", ids(got))
	}
}

func TestSelectProperties(t *testing.T) {
	rnd := rand.New(rand.NewSource(1))
	states := []State{Complete, Complete, Complete, Creating, Deleting, Missing}
	for n := 0; n < 3000; n++ {
		var ss []Snapshot
		for i := 0; i < rnd.Intn(30); i++ {
			ss = append(ss, Snapshot{
				ID: fmt.Sprintf("s%02d", i), Reason: Reasons[rnd.Intn(len(Reasons))], State: states[rnd.Intn(len(states))],
				Created: t0.Add(time.Duration(rnd.Intn(500)) * time.Hour), Pinned: rnd.Intn(5) == 0,
			})
		}
		p := Policy{Keep: rnd.Intn(8), MaxAge: time.Duration(rnd.Intn(300)) * time.Hour, MinKeep: rnd.Intn(5),
			KeepByReason: map[Reason]int{Reasons[rnd.Intn(len(Reasons))]: rnd.Intn(4)}}
		got := Select(ss, p, t0.Add(500*time.Hour))
		if again := Select(ss, p, t0.Add(500*time.Hour)); ids(again) != ids(got) {
			t.Fatal("Select must be deterministic")
		}
		deleted := map[string]bool{}
		for _, d := range got {
			deleted[d.ID] = true
			if d.Pinned || d.State != Complete {
				t.Fatalf("selected a pinned or not complete snapshot: %+v", d)
			}
		}
		var complete []Snapshot
		for _, s := range ss {
			if s.State == Complete {
				complete = append(complete, s)
			}
		}
		if len(complete) == 0 {
			continue
		}
		newest := complete[0]
		for _, s := range complete {
			if s.Created.After(newest.Created) {
				newest = s
			}
		}
		for _, s := range complete { // a tie for newest: at least one of them stays
			if s.Created.Equal(newest.Created) && !deleted[s.ID] {
				newest = s
				break
			}
		}
		if deleted[newest.ID] {
			t.Fatalf("the newest complete snapshot must stay: %+v policy %+v", newest, p)
		}
		if left := len(complete) - len(got); left < min(p.MinKeep, len(complete)) {
			t.Fatalf("%d complete snapshots left, min_keep %d (had %d)", left, p.MinKeep, len(complete))
		}
	}
}

func TestOldestForSpace(t *testing.T) {
	ss := snaps(Manual, Manual, Manual, Manual)
	ss[1].Pinned = true
	if got := Oldest(ss, 2); ids(got) != "s00 s02 " {
		t.Errorf("room for min_keep 2 of 4: the unpinned old ones but never the newest, got %s", ids(got))
	}
	if got := Oldest(ss, 4); len(got) != 0 {
		t.Errorf("min_keep 4 of 4 frees nothing: %s", ids(got))
	}
	if got := Oldest(nil, 1); got != nil {
		t.Error("nothing to free")
	}
}

func TestValidReason(t *testing.T) {
	if !ValidReason(Manual) || ValidReason("whatever") {
		t.Error("ValidReason")
	}
}
