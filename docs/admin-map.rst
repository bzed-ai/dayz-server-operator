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

* direct messages and broadcasts (in chat, as an important message, or as an
  on-screen notification),
* teleport (to a map position or to another player),
* spawn items into a player's inventory, hands or on the ground,
* repair or delete vehicles.

The mod connects *out* to dzo: about once a second it posts what changed and the
results of finished actions to ``dzo serve``, and gets the next actions in the
reply. Nothing connects into the game server. The mod only knows persistent
vehicles of the car type (``CarScript``, which includes most modded cars); boats
and helicopters are not reported yet.

Installing it
~~~~~~~~~~~~~

dzo-admin is a local server mod, not a Steam Workshop mod. Put it in
``site.yaml`` and enable it per instance:

.. code-block:: yaml

   # site.yaml
   local_mods:
     dzo-admin: {url: "https://<release host>/dzo-admin-0.1.0.tar.gz", sha256: "<hash>"}

   # instances/deerisle/instance.yaml
   mods:
     - {local: dzo-admin, server: true}
   admin:
     deny_classes: ["Land_*"]

Run ``dzo mod add dzo-admin --instance deerisle --server`` to install it, then
start the instance. dzo recognises the mod by the name ``dzo-admin``.

Every time the instance starts, ``dzo instance render`` writes the mod's
configuration to ``profiles/dzo-admin/config.json`` with a **new token** (the
file is readable by the owner only and is not part of the site repository). The
token is the only protection of the endpoint, so keep ``serve.mod_listen`` on
the local host. With ``network: host`` the mod connects to ``127.0.0.1``; with
``network: publish`` to ``host.containers.internal``.

Settings in ``instance.yaml`` under ``admin`` (see :doc:`configuration`):

* ``sync_ms``, ``players_s``, ``vehicles_s``, ``markers_s``, ``events_s``: how
  often the mod polls for actions and pushes each kind of state;
* ``allow_spawn``, ``allow_classes``, ``deny_classes``: which item classes
  admins may spawn. A class must be a valid class name, must not match a deny
  entry, and, if an allow list exists, must match it. dzo also rejects classes
  the server did not report.

Without dzo-admin the web interface cannot show the map or run actions. Other
features that do not need it (status, RCon) are unaffected.

Check that it works: ``dzo player list <instance>`` shows the online players
once the mod has connected, and the instance page shows "dzo-admin connected".

Map background
--------------

The map draws a 1 km grid on a plain background. A background built from the
map's terrain data is planned but not available yet.

Active events
-------------

dzo-admin reports effect areas (for example contaminated zones) with their
position and radius, and events that other mods publish through the marker API.
They are shown as a map layer and available at ``/events`` in the API. Events of
the Central Economy (helicopter crashes, convoys) are not detected yet; add
them with watch rules below until they are.

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
       - {layer: campfire,  classes: ["FireplaceBase"], icon: fire, max: 200}
       - {layer: ufo_crash, classes: ["UFO_Crash_Site*"], icon: ufo, label: "UFO"}
       - {layer: ai_convoy, classes: ["eAIBase"], icon: ai, cluster: 50}

Subclasses match too, and a ``*`` at the end of a class name matches a prefix.
``max`` limits the markers of a layer. ``cluster`` is a hint for the map
display. ``only_if`` is not supported yet and is rejected, so that it is never
silently ignored.

2. Script API
~~~~~~~~~~~~~

A mod calls the API only when dzo-admin is present:

.. code-block:: c

   DZOAdmin_Map.Upsert("ufo_crash", m_CrashId, GetPosition(), "ufo", "UFO crash", props, 3600);
   DZOAdmin_Map.Track("ai_convoy", convoyLeader, "truck", "Convoy " + m_Name);  // follows the entity
   DZOAdmin_Map.Remove("ufo_crash", m_CrashId);

Functions: ``Upsert``, ``Circle``, ``Path``, ``Track``, ``Untrack``, ``Remove``,
``ClearLayer`` and ``DefineLayer``. dzo-admin defines ``DZO_ADMIN`` for scripts,
so the ``#ifdef`` above compiles only when it is installed. Do not list it in
your mod's ``requiredAddons``. The dzo-admin repository has a small example mod
(``examples/marker-demo``) that shows how to call the API without a hard
dependency.

3. File drop
~~~~~~~~~~~~

Write ``$profile:dzo-admin/markers/<layer>.json``, either a JSON list of
markers or ``{"markers": [...]}``. The layer name defaults to the file name,
and dzo re-reads the files every few seconds. This works for any mod or
external tool, and also when the mod cannot use ``#ifdef``.

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

Markers carry an icon name, which is shown in the tooltip; the map draws every
marker as a coloured shape today. Built-in icons and icons shipped inside mod
PBOs are planned.
