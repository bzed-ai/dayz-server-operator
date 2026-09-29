// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package product

import (
	"path/filepath"
	"strconv"

	"github.com/bzed-ai/dayz-server-operator/internal/cache"
)

// ProductStore returns the internal/cache.Store that holds one product's
// downloaded server build generations: cache/products/<product>/<buildid>/
// (§C2/§C7). It does not touch the filesystem; call EnsureRoot before
// first use, as with any cache.Store.
func ProductStore(cacheRoot, product string) *cache.Store {
	return cache.New(filepath.Join(cacheRoot, "products", product))
}

// ModStore returns the internal/cache.Store that holds one workshop mod's
// downloaded generations: cache/workshop/<workshopAppID>/<modID>/<time_updated>/
// (§C7). Mods are shared between products that use the same workshop app,
// so this is keyed by workshopAppID + modID, not by product.
func ModStore(cacheRoot string, workshopAppID uint32, modID uint64) *cache.Store {
	return cache.New(filepath.Join(
		cacheRoot, "workshop", strconv.FormatUint(uint64(workshopAppID), 10), strconv.FormatUint(modID, 10),
	))
}

// ModGenerationID is the generation id a downloaded mod version is stored
// under: its Steam Web API time_updated value (§C7 - deliberately not
// local mtime, D14). A forced refresh of an already-current time_updated
// needs a distinct id (the old generation may still be referenced by a
// running server); retry, starting at 1, produces that: "<time_updated>",
// "<time_updated>-r1", "<time_updated>-r2", ...
func ModGenerationID(timeUpdated int64, retry int) string {
	id := strconv.FormatInt(timeUpdated, 10)
	if retry > 0 {
		id += "-r" + strconv.Itoa(retry)
	}
	return id
}

// LocalModStore is the store of a local servermod's content-hashed
// generations, cache/local/<name>/ (§C7, D37).
func LocalModStore(cacheRoot, name string) *cache.Store {
	return cache.New(filepath.Join(cacheRoot, "local", name))
}
