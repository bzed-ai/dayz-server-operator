.. SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
.. SPDX-License-Identifier: AGPL-3.0-or-later

Missions, integrations and overlays
===================================

The mission folder holds the Central Economy files, ``init.c``, and the game
world (``storage_1``). dzo treats it with care: **the live mission is never
wiped or recreated**.

Pristine and live
-----------------

Each instance has two copies:

``servermpmissions/`` — the pristine mission
   A checkout of the instance's mission repository at the configured ``ref``.
   ``dzo mission update <name>`` fetches a new version. Nothing else changes it.

``mpmissions/<map>/`` — the live mission
   What the server runs. It is created once from the pristine mission
   (``dzo instance create`` or ``dzo mission init``) and then only updated in
   place.

Which files dzo manages
-----------------------

dzo keeps a manifest of the files it owns in the live mission:

Managed files
   Every file that comes from the pristine mission. dzo updates them on every
   render.

Generated files
   Folders dzo creates itself: ``mod_<id>/`` for mod CE data, ``custom_<name>/``
   for overlays, and EditorFiles.

Foreign files
   Everything else: ``storage_*``, files mods write at runtime, files you
   placed by hand. **dzo never changes or deletes them.** Paths listed in
   ``mission.unmanaged`` are treated as foreign even if the mission repository
   ships them.

If a managed file was changed in the live mission (by a mod or by hand), dzo
warns, copies the changed file to ``filehistory/`` and overwrites it
(``drift: warn-backup-overwrite``). Recurring drift on the same file usually
means it belongs in ``mission.unmanaged``.

The render
----------

Before every start dzo builds the mission in a staging area:

#. start from the pristine mission;
#. add each mod's integration files, in the order of the ``mods`` list;
#. add overlays (shared ones first, then the instance's own);
#. validate everything: XML and JSON must parse, every referenced file and
   every CE folder must exist;
#. only if everything is valid: take a snapshot if one is due, write the changes
   into the live mission atomically, update the manifest.

If validation fails, the start is aborted and the live mission is untouched.

Preview a render at any time:

.. code-block:: sh

   dzo instance render deerisle --dry-run

``dzo instance render <name>`` (without ``--dry-run``) is the same render, applied.
It is what the unit runs before every start. The first render into an empty
``mpmissions/<map>`` is the one-time initialisation from the pristine mission;
every later render only touches files dzo manages (in the manifest, or new in
the pristine mission). ``storage_*`` and everything matching
``mission.unmanaged`` is never written.

The pristine mission is fetched from ``mission_source`` (a git repo, ``ref`` and
``path``) the first time it is missing, and never again on its own, so a start
does not need the network. ``--update-pristine`` fetches it again; the live
mission follows on that same render. A base file the map lacks (for example
``cfgweather.xml``) is taken from ``fallback_mission`` in the installed server
build, when a build is installed.

How mod files are merged
------------------------

``integrations/mods/<modid>/integration.yaml`` says which files a mod
contributes and where they come from:

.. code-block:: yaml

   mod: 2291785308
   name: DayZExpansionCore
   files:
     types.xml:             {source: local, path: files/types.xml}
     cfgspawnabletypes.xml: {source: url, url: https://…, sha256: …}
     cfgeventspawns.xml:    {source: local, path: files/cfgeventspawns.xml,
                             maps: [dayzOffline.chernarusplus]}
     events.xml:            {source: mod, path: ./info/events.xml}
   hooks:
     post_merge: [hooks/remove-static-trains.sh]

``source`` is ``local`` (in the site repository), ``url`` (downloaded, cached,
optionally pinned by hash) or ``mod`` (a file shipped inside the mod). dzo never
changes the downloaded mod itself.

Each file type has a fixed merge strategy:

.. list-table::
   :header-rows: 1

   * - Files
     - Strategy
   * - ``types``, ``spawnabletypes``, ``events``, ``globals``
     - copied to ``mod_<id>/`` and registered as a ``<ce folder>`` in
       ``cfgeconomycore.xml``
   * - ``mapgrouppos``, ``mapgroupproto``, ``cfgeventgroups``,
       ``cfgeventspawns``, ``cfgenvironment``, ``cfgrandompresets``,
       ``zombie_territories``
     - XML elements appended to the mission file
   * - ``cfggameplay.json``
     - deep merge; known lists (object spawners, spawn gear presets,
       restricted areas) are appended without duplicates
   * - ``cfgundergroundtriggers.json``, ``cfgeffectarea.json``
     - arrays concatenated
   * - ``cfgweather.xml``, ``messages.xml``
     - replaced
   * - ``init.c``
     - patched
   * - anything else
     - copied to ``custom_<name>/``

When two mods define the same thing, the one later in the ``mods`` list wins and
the render report lists the conflict.

Overlays
--------

An overlay is a folder of mission files: your own tweaks, loadouts, object
spawners. Shared overlays live in ``site/overlays/``, instance overlays in
``instances/<name>/overlays/``. They use the same merge strategies as mods.

An ``overlay.yaml`` can declare files that ``cfggameplay.json`` must reference.
dzo then adds the references with the right path:

.. code-block:: yaml

   object_spawners: ["*.json"]
   spawn_gear_presets: ["loadout-*.json"]

Hooks
-----

For anything the strategies cannot express, a hook runs a script:
``post_merge`` hooks run in the staging area, ``pre_start`` hooks run against
the live mission after it was updated (for example to update trader stock).
Hooks get ``DZO_STAGING``, ``DZO_LIVE_MISSION``, ``DZO_INSTANCE`` and ``DZO_MAP``
in their environment and a JSON context on stdin. A non-zero exit aborts the
start.

Mission history and reset
-------------------------

* ``dzo mission rollback <name> [<time>]`` restores managed files from
  ``filehistory/``.
* ``dzo mission reinit <name>`` recreates the live mission from pristine. This is
  the only destructive mission command: it asks for confirmation and always
  takes a snapshot first.
