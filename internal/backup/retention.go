// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package backup

import (
	"sort"
	"time"
)

// Reason says why a snapshot was taken.
type Reason string

// The reasons of §C20. Replaced and PreRestore are the ones a restore makes.
const (
	Update        Reason = "update"
	ModUpdate     Reason = "mod_update"
	MissionUpdate Reason = "mission_update"
	ConfigChange  Reason = "config_change"
	Render        Reason = "render"
	Scheduled     Reason = "scheduled"
	Manual        Reason = "manual"
	Destructive   Reason = "destructive"
	PreRestore    Reason = "pre_restore"
	Replaced      Reason = "replaced"
)

// Reasons lists every reason.
var Reasons = []Reason{Update, ModUpdate, MissionUpdate, ConfigChange, Render, Scheduled, Manual, Destructive, PreRestore, Replaced}

// ValidReason reports whether r is a reason of this package.
func ValidReason(r Reason) bool {
	for _, x := range Reasons {
		if x == r {
			return true
		}
	}
	return false
}

// State is where a snapshot is in its life.
type State string

// The states of a snapshot in the index.
const (
	Creating State = "creating"
	Complete State = "complete"
	Deleting State = "deleting"
	Missing  State = "missing"
)

// Snapshot is one entry of the index. Its directory is Root/ID.
type Snapshot struct {
	ID      string    `json:"id"`
	Reason  Reason    `json:"reason"`
	State   State     `json:"state"`
	Created time.Time `json:"created"`
	Pinned  bool      `json:"pinned,omitempty"`
}

// Policy is the retention of one instance (site.BackupConfig, resolved).
type Policy struct {
	Keep         int
	KeepByReason map[Reason]int
	MaxAge       time.Duration
	MinKeep      int
	MinFreeBytes int64
}

// Select returns the snapshots to delete, oldest first. It is a pure function:
//   - only complete, unpinned snapshots are ever candidates,
//   - the newest complete snapshot is never deleted,
//   - the number of complete snapshots never drops below MinKeep (when there
//     are that many), whatever the other limits say.
//
// A snapshot is a candidate if it is over the limit of its reason, older than
// MaxAge, or over the total Keep (oldest first).
func Select(snaps []Snapshot, p Policy, now time.Time) []Snapshot {
	var complete []Snapshot
	for _, s := range snaps {
		if s.State == Complete {
			complete = append(complete, s)
		}
	}
	sort.SliceStable(complete, func(i, j int) bool { return complete[i].Created.Before(complete[j].Created) })
	if len(complete) == 0 {
		return nil
	}

	marked := map[string]bool{}
	eligible := func(s Snapshot) bool { return !s.Pinned && !marked[s.ID] }

	for reason, keep := range p.KeepByReason {
		var of []Snapshot // newest first
		for i := len(complete) - 1; i >= 0; i-- {
			if complete[i].Reason == reason && !complete[i].Pinned {
				of = append(of, complete[i])
			}
		}
		for i := keep; i < len(of) && keep >= 0; i++ {
			marked[of[i].ID] = true
		}
	}
	if p.MaxAge > 0 {
		for _, s := range complete {
			if eligible(s) && now.Sub(s.Created) > p.MaxAge {
				marked[s.ID] = true
			}
		}
	}
	remaining := len(complete) - len(marked)
	for _, s := range complete { // oldest first
		if p.Keep > 0 && remaining > p.Keep && eligible(s) {
			marked[s.ID] = true
			remaining--
		}
	}

	// the protections: undo deletions, newest first
	newest := complete[len(complete)-1]
	delete(marked, newest.ID)
	for i := len(complete) - 1; i >= 0 && len(complete)-len(marked) < p.MinKeep; i-- {
		delete(marked, complete[i].ID)
	}

	var out []Snapshot
	for _, s := range complete {
		if marked[s.ID] {
			out = append(out, s)
		}
	}
	return out
}

// Oldest returns the snapshots that may be deleted to make room, oldest first:
// complete and unpinned, without the newest complete snapshot, and without
// going below MinKeep complete snapshots in total.
func Oldest(snaps []Snapshot, minKeep int) []Snapshot {
	var complete []Snapshot
	for _, s := range snaps {
		if s.State == Complete {
			complete = append(complete, s)
		}
	}
	sort.SliceStable(complete, func(i, j int) bool { return complete[i].Created.Before(complete[j].Created) })
	if len(complete) == 0 {
		return nil
	}
	var out []Snapshot
	for _, s := range complete[:len(complete)-1] { // never the newest
		if !s.Pinned && len(complete)-len(out) > minKeep {
			out = append(out, s)
		}
	}
	return out
}
