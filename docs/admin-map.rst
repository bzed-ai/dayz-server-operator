.. SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
.. SPDX-License-Identifier: AGPL-3.0-or-later

Admin map and the dzo-admin mod
===============================

The web interface has a live map of each server: players, vehicles, active
events and markers from other mods. Clicking the map teleports players, and the
map offers actions on players and vehicles.

The dzo-admin server mod
------------------------

Everything that changes the game goes through **dzo-admin**, a small server-side
mod (clients do not need it). It reports players and vehicles and carries out
admin actions:

* direct and on-screen messages,
* teleport (to a map position or to another player),
* spawn items into a player's inventory, hands or on the ground,
* repair or delete vehicles.

Add it to an instance like any server mod (``server: true``). dzo writes its
configuration (endpoint and a per-server token) at every start. The mod connects
to dzo on the local host; nothing connects into the game server.

Without dzo-admin the web interface still works for status, players (from RCon),
kicks, bans and broadcasts, but not for the map or item spawns.

Map tiles
---------

The map background is generated from the map's own terrain data (a data PBO).
There are no third-party map tiles.

Vanilla maps (Chernarus, Livonia, Sakhal)
   The dedicated server package only contains empty placeholder textures, so dzo
   downloads the one data PBO it needs from the DayZ client depot with your Steam
   login (about 200–300 MB per map). If that fails, upload the file yourself
   (see below).

Modded maps (e.g. DeerIsle, Namalsk)
   You must provide the map's data PBO, the one that contains
   ``layers/S_*_lco.paa``. Upload it in the web interface or set it on the host:

   .. code-block:: sh

      dzo map source set deerisle /path/to/deerisle/data.pbo

   or point to it in ``/etc/dzo/config.yaml``:

   .. code-block:: yaml

      map_tiles:
        maps:
          deerisle: {source: /srv/dayz/mapsources/deerisle-data.pbo}

A server whose map has no source simply shows no map background.

dzo checks every source file and refuses the stripped server copy with a clear
message. Tiles are built once per file and reused by all servers on that map.
They are rebuilt when the source changes:

.. code-block:: sh

   dzo map tiles status [<map>]
   dzo map source update <map>        # retry the automatic download (vanilla maps)
   dzo map tiles update <map> [--force]
   dzo map tiles export <map> <dir>   # for your own tile server

Active events
-------------

dzo-admin reports which Central Economy events are active and where: helicopter
crashes, convoys, contaminated areas, trains, and modded events. They appear as a
map layer and can be sent to Discord. dzo derives which objects belong to which
event from the server's own ``events.xml``, so events added by mods work
without extra configuration.

Markers from other mods
-----------------------

Other mods can put their content on the admin map (UFO crashes, AI convoys,
camp fires, trader zones, …) without depending on dzo-admin. They keep working
when dzo-admin is not installed. There are three ways, from easiest to most
flexible.

1. Watch rules (no code)
~~~~~~~~~~~~~~~~~~~~~~~~

Configure per instance which object classes appear on the map:

.. code-block:: yaml

   admin_map:
     watch:
       - {layer: campfire,  classes: ["FireplaceBase"], icon: fire, only_if: burning}
       - {layer: ufo_crash, classes: ["UFO_Crash_Site*"], icon: ufo, label: "UFO"}
       - {layer: ai_convoy, classes: ["eAIBase"], icon: ai, cluster: 50}

2. Script API
~~~~~~~~~~~~~

A mod calls the API only when dzo-admin is present:

.. code-block:: c

   DZOAdmin_Map.Upsert("ufo_crash", m_CrashId, GetPosition(), "ufo", "UFO crash", props, 3600);
   DZOAdmin_Map.Track("ai_convoy", convoyLeader, "truck", "Convoy " + m_Name);  // follows the entity
   DZOAdmin_Map.Remove("ufo_crash", m_CrashId);

Functions: ``Upsert``, ``Track``, ``Untrack``, ``Remove``, ``ClearLayer`` and
``DefineLayer``. The dzo-admin repository has a small example mod that shows how
to call the API without a hard dependency.

3. File drop
~~~~~~~~~~~~

Write ``$profile:dzo-admin/markers/<layer>.json``. This works for any mod or
external tool.

Marker fields
~~~~~~~~~~~~~

.. list-table::
   :header-rows: 1

   * - Field
     - Meaning
   * - ``layer``
     - category; each layer is a toggle on the map
   * - ``id``
     - unique within the layer
   * - ``shape``
     - ``point`` (default), ``circle``, ``polygon`` or ``polyline``
   * - ``position`` / ``points``
     - world coordinates (x, z)
   * - ``entity``
     - follow an object; the marker moves with it and disappears with it
   * - ``icon``, ``color``, ``label``
     - presentation
   * - ``props``
     - key/value pairs shown in the tooltip
   * - ``ttl``
     - lifetime in seconds
   * - ``visibility``
     - lowest role that sees the marker (``admin`` by default)

Icons
~~~~~

dzo has built-in icons (player, vehicles, fire, UFO, AI, loot, zone, event). A
mod can ship its own icons as SVG or PNG files at ``<prefix>/dzo_icons/<name>.svg``
inside its PBO; they become available as ``<modname>:<name>``. The site
repository can add or override icons in ``map_icons/``.
