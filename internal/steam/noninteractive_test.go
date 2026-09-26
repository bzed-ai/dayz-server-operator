// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package steam

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNonInteractivePrompterRefusesPassword(t *testing.T) {
	var p NonInteractivePrompter
	_, err := p.Password(context.Background(), "bob")
	if !errors.Is(err, ErrAuthRequired) {
		t.Errorf("err = %v, want ErrAuthRequired", err)
	}
}

func TestNonInteractivePrompterRefusesCode(t *testing.T) {
	var p NonInteractivePrompter
	_, err := p.Code(context.Background(), PromptGuardCode)
	if !errors.Is(err, ErrAuthRequired) {
		t.Errorf("err = %v, want ErrAuthRequired", err)
	}
}

func TestNonInteractivePrompterWaitingIsNoop(t *testing.T) {
	var p NonInteractivePrompter
	p.Waiting(context.Background()) // must not panic or block
}

func TestRunWithNonInteractivePrompterReportsAuthRequired(t *testing.T) {
	script := writeFakeSteamcmd(t, `
printf 'password: '
read pass
echo "Waiting for user info...OK"
`)
	orig := killGraceDelay
	killGraceDelay = 100 * time.Millisecond
	defer func() { killGraceDelay = orig }()

	_, err := Run(context.Background(), runOpts(t, script, NonInteractivePrompter{}))
	if !errors.Is(err, ErrAuthRequired) {
		t.Errorf("err = %v, want ErrAuthRequired", err)
	}
}
