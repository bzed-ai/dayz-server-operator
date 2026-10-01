// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

// Live admin map. World coordinates are metres: x east, z north, so Leaflet's
// simple CRS maps lng = x and lat = z. There is no background tile layer yet,
// only a 1 km grid. All text from the server goes in through textContent.
(function () {
  'use strict';
  var el = document.getElementById('map');
  if (!el || typeof L === 'undefined') { return; }
  var size = parseInt(el.dataset.size, 10) || 15360;
  var base = el.dataset.base;
  var csrf = document.querySelector('meta[name=csrf]').content;

  var map = L.map(el, { crs: L.CRS.Simple, minZoom: -5, maxZoom: 2, zoomSnap: 0.25 });
  map.fitBounds([[0, 0], [size, size]]);
  L.rectangle([[0, 0], [size, size]], { color: '#6b7f5e', weight: 1, fill: false, interactive: false }).addTo(map);
  for (var g = 1000; g < size; g += 1000) {
    L.polyline([[g, 0], [g, size]], { color: '#00000022', weight: 1, interactive: false }).addTo(map);
    L.polyline([[0, g], [size, g]], { color: '#00000022', weight: 1, interactive: false }).addTo(map);
  }

  var players = L.layerGroup().addTo(map);
  var vehicles = L.layerGroup().addTo(map);
  var events = L.layerGroup().addTo(map);
  var control = L.control.layers(null, { Players: players, Vehicles: vehicles, Events: events }, { collapsed: false }).addTo(map);
  var markerLayers = {};
  var playerList = [];

  function ll(o) { return [o.z, o.x]; }

  function text(tag, s) {
    var n = document.createElement(tag);
    n.textContent = s;
    return n;
  }

  function post(path, params) {
    return fetch(base + path, {
      method: 'POST',
      headers: { 'X-CSRF-Token': csrf, 'Content-Type': 'application/x-www-form-urlencoded' },
      body: new URLSearchParams(params)
    }).then(function (r) { return r.text(); });
  }

  function flash(html) {
    var f = document.getElementById('flash');
    if (f) { f.innerHTML = html; } // the server's own flash fragment, already escaped
  }

  function tooltip(title, lines) {
    var d = document.createElement('div');
    d.appendChild(text('strong', title));
    (lines || []).forEach(function (l) { d.appendChild(document.createElement('br')); d.appendChild(text('span', l)); });
    return d;
  }

  function onPlayers(list) {
    playerList = list;
    players.clearLayers();
    list.forEach(function (p) {
      var m = L.circleMarker(ll(p), { radius: 6, color: p.alive ? '#1b5fd1' : '#888', fillOpacity: 0.8 });
      m.bindTooltip(tooltip(p.name, [p.steam_id, 'health ' + Math.round(p.health)]));
      m.on('click', function () {
        var d = tooltip(p.name, [p.steam_id]);
        var b = text('button', 'Actions');
        b.addEventListener('click', function () { window.location = base; });
        d.appendChild(document.createElement('br'));
        d.appendChild(b);
        m.bindPopup(d).openPopup();
      });
      m.addTo(players);
    });
  }

  function onVehicles(list) {
    vehicles.clearLayers();
    list.forEach(function (v) {
      var r = 25;
      var m = L.rectangle([[v.z - r, v.x - r], [v.z + r, v.x + r]], { color: v.ruined ? '#777' : '#c26a00', weight: 2 });
      m.bindTooltip(tooltip(v.type, ['health ' + Math.round(v.health * 100) + '%', 'fuel ' + Math.round(v.fuel * 100) + '%', 'crew ' + (v.occupants || []).length]));
      m.addTo(vehicles);
    });
  }

  function onEvents(list) {
    events.clearLayers();
    list.forEach(function (e) {
      var m = L.circle(ll(e), { radius: e.radius || 50, color: '#b3261e', weight: 2, fillOpacity: 0.15 });
      m.bindTooltip(tooltip(e.name, [e.category]));
      m.addTo(events);
    });
  }

  function onMarkers(list) {
    Object.keys(markerLayers).forEach(function (k) { markerLayers[k].clearLayers(); });
    list.forEach(function (m) {
      var group = markerLayers[m.layer];
      if (!group) {
        group = L.layerGroup().addTo(map);
        markerLayers[m.layer] = group;
        control.addOverlay(group, m.layer);
      }
      var color = m.color || '#7a3db8';
      var shape;
      if (m.shape === 'circle') {
        shape = L.circle(ll(m), { radius: m.radius || 50, color: color, weight: 2, fillOpacity: 0.15 });
      } else if ((m.shape === 'polygon' || m.shape === 'polyline') && m.points && m.points.length) {
        var pts = m.points.map(function (p) { return [p.z, p.x]; });
        shape = m.shape === 'polygon' ? L.polygon(pts, { color: color }) : L.polyline(pts, { color: color });
      } else {
        shape = L.circleMarker(ll(m), { radius: 5, color: color, fillOpacity: 0.9 });
      }
      var lines = (m.props || []).map(function (kv) { return kv.k + ': ' + kv.v; });
      shape.bindTooltip(tooltip(m.label || m.id, lines));
      shape.addTo(group);
    });
  }

  // Click on the map: offer to teleport a player there.
  map.on('click', function (ev) {
    if (!playerList.length) { return; }
    var x = Math.round(ev.latlng.lng), z = Math.round(ev.latlng.lat);
    var form = document.createElement('form');
    form.appendChild(text('div', 'x ' + x + ' / z ' + z));
    var sel = document.createElement('select');
    playerList.forEach(function (p) {
      var o = text('option', p.name);
      o.value = p.steam_id;
      sel.appendChild(o);
    });
    form.appendChild(sel);
    form.appendChild(text('button', 'Teleport here'));
    form.addEventListener('submit', function (e) {
      e.preventDefault();
      post('/players/' + encodeURIComponent(sel.value) + '/teleport', { x: x, z: z }).then(flash);
      map.closePopup();
    });
    L.popup().setLatLng(ev.latlng).setContent(form).openOn(map);
  });

  var handlers = { players: onPlayers, vehicles: onVehicles, events: onEvents, markers: onMarkers };
  var es = new EventSource(el.dataset.stream);
  Object.keys(handlers).forEach(function (k) {
    es.addEventListener(k, function (e) { handlers[k](JSON.parse(e.data) || []); });
  });
})();
