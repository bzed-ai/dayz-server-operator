// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package product implements the parts of the update engine (§C7) that
// are fully specifiable and testable without a live Steam account or a
// real DayZ install:
//
//   - Schedule: the update-window/quiet-hours/batching/min-restart-interval
//     decision ("should the pending mod updates for this instance be
//     applied right now?"), evaluated against an instance's
//     internal/site.UpdatesConfig.
//   - Freshness: a Steam Web API client for GetPublishedFileDetails
//     (workshop mod time_updated), the signal that decides whether a mod
//     has a new version - deliberately not local mtime (D14).
//   - Job: running steamcmd non-interactively for the two operations the
//     update engine needs (+app_update, +workshop_download_item),
//     classifying their own success/failure output. A job's login phase
//     reuses internal/steam.Run with a NonInteractivePrompter: a cached
//     Steam session should let it proceed without any prompt, and if
//     steamcmd prompts anyway, the job fails fast with
//     steam.ErrAuthRequired rather than hanging.
//   - Snapshot: naming and creating the immutable internal/cache
//     generations a downloaded build or mod version is stored into.
//
// Deliberately out of scope for now, and left as a documented gap rather
// than half-built: mapping a downloaded mod update to the instances that
// use it, actually triggering a restart, and garbage-collecting
// generations no running instance references - all of which need
// internal/instance (not built yet) to know what instances exist and
// what they currently reference. Mod dependency (CfgPatches) parsing is
// internal/moddeps. The forced-refresh steamcmd-cache-invalidation
// procedure (§C7) needs a verified steamcmd work-dir layout and is not
// implemented here yet either.
//
// Nothing in this package has been exercised against a live Steam
// account or the real Steam Web API from this environment (no network
// access to Steam here); the success/failure wording each job classifies
// against is either quoted directly from the plan's own record of the
// legacy implementation's behaviour ("Success. Downloaded item <id>") or
// widely documented steamcmd usage ("Success! App '<id>' fully
// installed."), but neither has been independently re-verified here.
package product
