// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package steam

import (
	"context"
	"errors"
)

// ErrAuthRequired is returned by NonInteractivePrompter's methods: any
// prompt appearing while it is in use means the cached Steam session is
// stale or invalid, and there is no human attached to answer it.
var ErrAuthRequired = errors.New("steam: interactive input required but no operator is attached (run `dzo steam login`)")

// NonInteractivePrompter is a Prompter for unattended callers - the update
// engine's scheduled steamcmd jobs (§C7): a cached Steam session should let
// those run without any prompt at all. If steamcmd prompts anyway, dzo
// cannot answer on a human's behalf, so Password and Code fail immediately
// with ErrAuthRequired instead of hanging until Run's context expires.
// Waiting is left as a no-op: an app-mobile-confirmation wait has no
// input to refuse, so the caller's context timeout is what eventually
// ends an unattended job stuck on one.
type NonInteractivePrompter struct{}

func (NonInteractivePrompter) Password(context.Context, string) (string, error) {
	return "", ErrAuthRequired
}

func (NonInteractivePrompter) Code(context.Context, PromptKind) (string, error) {
	return "", ErrAuthRequired
}

func (NonInteractivePrompter) Waiting(context.Context) {}
