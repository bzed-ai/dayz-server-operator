#!/bin/sh
# SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
# SPDX-License-Identifier: AGPL-3.0-or-later
#
# Strips the packages/files listed in .coverage-exclude out of a go test
# coverage profile, so the 85% gate (D20) is measured only over code meant
# to be tested.
set -eu

in="${1:?usage: filter-coverage.sh <in.out> <out.out>}"
out="${2:?usage: filter-coverage.sh <in.out> <out.out>}"
exclude_file="$(dirname "$0")/../.coverage-exclude"

head -n1 "$in" > "$out"

if [ -f "$exclude_file" ]; then
	pattern=$(grep -vE '^\s*#|^\s*$' "$exclude_file" | paste -sd'|' -)
else
	pattern=""
fi

if [ -n "$pattern" ]; then
	tail -n +2 "$in" | grep -vE "$pattern" >> "$out" || true
else
	tail -n +2 "$in" >> "$out"
fi
