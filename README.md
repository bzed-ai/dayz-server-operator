<!--
SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
SPDX-License-Identifier: AGPL-3.0-or-later
-->

<div align="center">
  <img src="assets/logo.svg" alt="dzo logo" width="160" height="160">

  <h1>dzo</h1>
  <p><strong>A DayZ dedicated server operator for rootless podman.</strong></p>

  [![CI](https://github.com/bzed-ai/dayz-server-operator/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/bzed-ai/dayz-server-operator/actions/workflows/ci.yml)
  [![Codecov](https://codecov.io/gh/bzed-ai/dayz-server-operator/branch/main/graph/badge.svg)](https://codecov.io/gh/bzed-ai/dayz-server-operator)
  [![Go Report Card](https://goreportcard.com/badge/github.com/bzed-ai/dayz-server-operator)](https://goreportcard.com/report/github.com/bzed-ai/dayz-server-operator)
  [![Go Reference](https://pkg.go.dev/badge/github.com/bzed-ai/dayz-server-operator.svg)](https://pkg.go.dev/github.com/bzed-ai/dayz-server-operator)
  [![REUSE status](https://api.reuse.software/badge/github.com/bzed-ai/dayz-server-operator)](https://api.reuse.software/info/github.com/bzed-ai/dayz-server-operator)
  [![License: AGPL v3+](https://img.shields.io/badge/license-AGPL--3.0--or--later-blue.svg)](LICENSE)
</div>

---

`dzo` manages DayZ dedicated server instances on top of **rootless podman**
and **systemd quadlets** — no Node.js, no `podman generate systemd`, and no
external RCon tools. It's a single static Go binary that installs and
updates server builds and workshop mods into immutable, shareable
generations, renders each instance's mission and configuration from a
site-config repo, and talks to the game server directly over a built-in
BattlEye RCon client.

This replaces a fleet of ad-hoc bash scripts (`dzpodman`, `bercon-cli`,
`dayz_restart`, …) that grew organically across several production DayZ
servers, with one tool, one config model, and a test suite. The full
design — decisions, legacy analysis, architecture and the phased roadmap —
lives in [`IMPLEMENTATION_PLAN.md`](IMPLEMENTATION_PLAN.md); user-facing
documentation is a [Sphinx site](docs/) under `docs/`.

> **Status:** Phase 1 (core + CLI) is in progress. The packages below are
> implemented and tested; instance lifecycle (mission rendering, mod/product
> installs, quadlet-managed containers, backups, monitoring) is still being
> built. Track progress in the plan's [phased roadmap](IMPLEMENTATION_PLAN.md#part-e--phased-roadmap).

## What's here today

| Package | What it does |
|---|---|
| `internal/config` | Loads and validates `/etc/dzo/config.yaml` (data paths, products, notification targets) |
| `internal/servercfg` | Parses/writes `serverDZ.cfg` and diffs two versions before adoption |
| `internal/battleye` | A native BattlEye RCon client — login, commands, multi-packet responses, the connect/GUID/chat/kick event stream — no `bercon-cli`, no external RCon library |
| `internal/ce` | JSON deep-merge and XML child-merge primitives for combining mission/mod/overlay data, plus `cfgeconomycore.xml` `<ce folder>` registration |
| `internal/quadlet` | Renders podman quadlet `.container` units (health checks, host/publish networking, mounts) without relying on deprecated or newer-than-baseline podman tooling |
| `internal/cache` | An immutable, generation-based download cache with atomic activation and safe garbage collection |
| `cmd/dzo` | The CLI wiring all of the above together |

## Installing

**From a release build:** grab the `.deb` or a static binary from the
[latest CI run's artifacts](https://github.com/bzed-ai/dayz-server-operator/actions/workflows/ci.yml)
(tagged releases will publish these automatically once cut).

**From source** (needs a recent Go toolchain — see `go.mod`):

```console
$ git clone https://github.com/bzed-ai/dayz-server-operator.git
$ cd dayz-server-operator
$ make build
$ ./bin/dzo version
```

**As a Debian package** (targets Debian trixie):

```console
$ sudo apt install debhelper build-essential
$ dpkg-buildpackage -us -uc -b
$ sudo apt install ../dzo_*.deb
```

## Usage

```console
$ dzo version
0.1.0 (a1b2c3d, 2026-09-26T15:00:00Z)

$ dzo config validate --config /etc/dzo/config.yaml
config /etc/dzo/config.yaml is valid
  data:      /var/lib/dzo
  instances: /var/lib/dzo/instances
  products:  2 configured

$ dzo servercfg diff /profiles/serverDZ.cfg.save /files/serverDZ.cfg
~ hostname: "old name" -> "new name"
+ steamQueryPort

$ dzo rcon exec --addr 127.0.0.1:2303 --password s3cret players

$ dzo cache list /var/lib/dzo/cache/products/dayz-stable
* 1234567
  1234600

$ dzo quadlet render instance.yaml > dzo-myserver.container
```

Run `dzo --help` (or `<command> --help`) for the full flag reference.

## Developing

```console
$ make lint      # gofmt, go vet, golangci-lint
$ make reuse     # SPDX/REUSE licence header check
$ make test      # unit tests
$ make cover     # tests + the 85% coverage gate (D20)
$ make licenses  # dependency licence allow-list check (D15)
```

`.github/workflows/ci.yml` runs all of the above on every push, plus
cross-compiled binary and `.deb` builds. See
[`CONTRIBUTING` notes in the plan](IMPLEMENTATION_PLAN.md#c15-testing-strategy-and-ci-d20-d21)
for the full testing strategy.

## Licence

AGPL-3.0-or-later. This repository follows the [REUSE](https://reuse.software/)
specification — every file carries an SPDX header, and licence texts live
under [`LICENSES/`](LICENSES/). See [`LICENSE`](LICENSE) and
[`debian/copyright`](debian/copyright) for details.
