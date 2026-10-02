// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"github.com/bzed-ai/dayz-server-operator/internal/config"
	"github.com/bzed-ai/dayz-server-operator/internal/exporter"
)

// updateInfo is what the exporter reports about updates; the update engine
// will fill it from its state file.
func updateInfo(_ *config.Config) func() exporter.Updates {
	return func() exporter.Updates { return exporter.Updates{} }
}
