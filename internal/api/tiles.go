// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
)

var (
	tileMapRe  = regexp.MustCompile(`^[a-z0-9_-]{1,64}$`)
	tileFileRe = regexp.MustCompile(`^(metadata\.json|[0-9a-f]{8,64}/[0-9]{1,2}/[0-9]{1,6}/[0-9]{1,6}\.jpg)$`)
)

// ValidTilePath reports whether a map name and the rest of a tile URL are
// well-formed: metadata.json, or <hash>/<z>/<x>/<y>.jpg.
func ValidTilePath(name, rest string) bool {
	return tileMapRe.MatchString(name) && tileFileRe.MatchString(rest)
}

// tile serves GET /api/v1/tiles/<map>/metadata.json (the current tile set's
// metadata, which names its hash) and /api/v1/tiles/<map>/<hash>/<z>/<x>/<y>.jpg.
// The hash is in the path, so a tile never changes and may be cached for good.
func (s *Server) tile(w http.ResponseWriter, r *http.Request, _ *call) {
	name, rest := r.PathValue("map"), r.PathValue("rest")
	if s.TilesDir == "" || !ValidTilePath(name, rest) {
		fail(w, http.StatusNotFound, "no such tile")
		return
	}
	p := filepath.Join(s.TilesDir, name, "current", rest)
	cache := "private, max-age=60"
	if rest != "metadata.json" {
		p = filepath.Join(s.TilesDir, name, rest)
		cache = "private, max-age=31536000, immutable"
	}
	f, err := os.Open(p) //nolint:gosec // name and rest are validated above
	if err != nil {
		fail(w, http.StatusNotFound, "no such tile")
		return
	}
	defer func() { _ = f.Close() }()
	st, err := f.Stat()
	if err != nil || st.IsDir() {
		fail(w, http.StatusNotFound, "no such tile")
		return
	}
	w.Header().Set("Cache-Control", cache)
	if rest == "metadata.json" {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
	} else {
		w.Header().Set("Content-Type", "image/jpeg")
	}
	http.ServeContent(w, r, "", st.ModTime(), f)
}
