// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package api is the JSON API /api/v1 of one dzo installation (§C12). It is
// the only thing a client (the web interface, a bot, the dzo CLI) needs to
// talk to: every action in the web interface is one call here. The API names
// no host-specific path or id, so a client can use several installations at
// once by holding one base URL and token per installation (internal/apiclient).
//
// Authentication is by bearer token (TokenStore). Tokens carry a role and an
// optional list of instances; a token flagged "delegate" (the web interface)
// may name the human actor on whose behalf it acts, which goes into the audit
// log.
package api
