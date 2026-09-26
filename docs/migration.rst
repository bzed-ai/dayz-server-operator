Coming from dayzdockerserver
============================

dzo replaces ``dayzdockerserver``. There is **no data migration**: servers
moving to dzo start fresh, with a new world, empty profiles and no player or ban
history. The old servers are only used as a source of configuration.

Convert the configuration
-------------------------

``dzo legacy convert-config`` reads the ``dayzdockerserver`` git branches (never
podman volumes or a running host) and writes an example site repository for you
to review:

.. code-block:: sh

   dzo legacy convert-config \
       --repo ~/src/dayzdockerserver \
       --ref chernarus --ref livonia --ref deerisle \
       --out ~/src/dayz-site \
       [--port-offset 100] [--dry-run]

What it converts:

.. list-table::
   :header-rows: 1

   * - Old
     - New
   * - ``files/serverDZ.cfg``
     - ``instances/<name>/serverDZ.cfg`` (ports, ``template`` and
       ``instanceId`` move to ``instance.yaml``)
   * - container settings, ``dzpodman`` options
     - ``ports``, ``network``, ``container.mounts``/``env``, ``health``
   * - server parameters
     - ``params``
   * - map and mission settings
     - ``map``, ``mission_source``
   * - ``files/mods/@<Name>`` links
     - a **candidate** ``mods`` list, every entry marked ``# review: active?``
   * - ``files/servermods``
     - ``server: true`` on the matching mods
   * - ``files/mods/<id>/`` (``xml.env``, ``map.env``, XML/JSON files,
       ``init.c`` patches, ``start.sh``)
     - ``integrations/mods/<id>/`` (identical files across branches become
       shared integrations, differing ones per-instance overrides)
   * - ``files/custom/<dir>``
     - overlays (shared if identical across branches)
   * - ``files/messages.xml``
     - ``instances/<name>/messages.xml``
   * - ``pre_start.sh`` (trader stock, weather)
     - ``pre_start`` hooks

Not converted: which mods were actually active, restart times and update checks
(they lived in the host's crontab), RCon passwords, Steam credentials, and any
game data. Instances get the site defaults with ``# review`` markers instead.

Known problems of the old scripts (unreliable ``init.c`` patching, partial name
matching for server mods, a missing newline in ``xml.env``, …) are fixed in the
output. ``CONVERSION_REPORT.md`` lists per instance what was converted, what
needs a decision, what was dropped and why, and where branches disagree.

Review, then roll out
---------------------

#. Go through ``CONVERSION_REPORT.md`` and all ``# review`` markers. Settle the
   mod list and the mod order (the order decides which mod wins when two mods
   change the same mission file).
#. ``dzo site validate``.
#. For each server: announce the wipe → ``dzo instance create`` → install the
   build and mods → test in parallel on other ports (``--port-offset``) →
   switch to the real ports and stop the old server → remove the old setup
   once the new one is stable.

The old volumes are not touched and not read.
