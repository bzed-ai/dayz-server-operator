// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package instance ties the previously built packages together into one
// instance's lifecycle: materializing its systemd quadlet + timer units
// (internal/quadlet), the render-failure gate and restart-storm
// protection that make up F3 (§C5), starting/stopping/restarting it via
// systemd, and the graceful-restart announce/lock/kick sequence (§C8,
// port of the legacy dayz_restart) over internal/battleye.
//
// F3 ("no restart storms") has three layers; this package implements the
// two dzo itself is responsible for:
//
//  1. FailureGate: the "failed-render" gate keyed on an input hash. An
//     unplanned start whose pre-flight render fails is recorded; further
//     start attempts with the *same* input hash fail immediately without
//     doing the work again, until the inputs change or an operator acks
//     the failure. Running the actual pre-flight render is the caller's
//     job (internal/mission), since it needs a resolved staging tree this
//     package has no part in producing.
//  2. The Materialize output sets RestartLimitBurst/RestartLimitInterval
//     on the generated .container unit, so systemd's own start-rate
//     limiting (layer 3, entirely systemd's mechanism) is configured from
//     the instance's restart_limit.
//
// Deferred, documented rather than half-built: the optional
// restart_limit.cooldown retry timer (needs a scheduler this package does
// not have), btrfs snapshot integration around restarts (§C20), and
// mapping a mod update to the instances that use it (internal/product's
// documented gap, needs to know what instances reference which
// generations - which is exactly what this package would supply once a
// caller wires it up, but that wiring is not built yet either).
//
// Nothing here has been exercised against a real systemd user session,
// podman quadlet, or BattlEye server from this environment; Lifecycle and
// the graceful-restart sequence are tested against fake systemctl scripts
// and internal/battleye's own fake-server test harness respectively.
package instance
