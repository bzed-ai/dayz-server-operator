.. SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
.. SPDX-License-Identifier: AGPL-3.0-or-later

Configuration
=============

dzo has two kinds of configuration:

* the **operator configuration** in ``/etc/dzo/config.yaml``: paths, the site
  repository, products, notification targets, web and exporter settings;
* the **site repository**: everything about your servers, in git.

Secrets (Steam session, RCon passwords, webhook URLs, keys) are never in git.
They live in ``paths.secrets`` with mode 0600.

Operator configuration
----------------------

.. code-block:: yaml

   paths:
     data: /var/lib/dzo
     instances: ${data}/instances
     snapshots: ${data}/snapshots
     cache: ${data}/cache
     secrets: ${data}/secrets
     db: ${data}/db

   site:
     url: git@<git host>:<you>/dayz-site.git
     branch: main
     commit: true
     push: false

   products:                       # built-in defaults, override only if needed
     dayz-stable:       {server_appid: 223350,  branch: public, workshop_appid: 221100}
     dayz-experimental: {server_appid: 1042420, branch: public, workshop_appid: 221100}

   notify:
     discord:
       default: {}                 # the webhook URL is kept in the secrets directory
       admins: {events: [drift, render_failed, job_failed, steam_auth]}

   exporter:
     listen: ":9464"
     tls: {cert_file: null, key_file: null}

   database:
     driver: sqlite                # or: postgres
     dsn: null

Site repository layout
----------------------

.. code-block:: text

   site/
     site.yaml                      defaults for all instances
     integrations/
       mods/<modid>/integration.yaml   how a mod's files go into the mission
       mods/<modid>/files/…            local integration files
       mods/<modid>/hooks/…            scripts for this mod
       maps/<name>.yaml                mission source presets
     overlays/<name>/…              reusable mission overlays
     instances/<name>/
       instance.yaml
       serverDZ.cfg
       messages.xml
       overlays/<name>/…            overlays for this instance only
       integrations/<modid>/…       per-instance override of an integration
       hooks/…

``dzo site pull`` updates the checkout. Uncommitted local edits block the pull
and are reported instead of being overwritten. ``dzo site validate`` checks all
files against their schemas.

instance.yaml
-------------

.. code-block:: yaml

   name: deerisle
   product: dayz-stable            # or dayz-experimental
   map: empty.deerisle             # mission folder; must match serverDZ.cfg template
   mission_source:
     git: https://<git host>/<owner>/DeerIsle-mission.git
     ref: main                     # updated only with: dzo mission update
     path: "empty.deerisle"
   fallback_mission: dayzOffline.chernarusplus
   mission:
     unmanaged: ["expansion/**"]   # never managed, even if the mission repo ships it
     drift: warn-backup-overwrite
   ports: {game: 8302, rcon: 8303, query: 8716}
   network: host                   # or: publish
   params: {cpuCount: auto, extra: ["-netlog", "-adminlog"]}
   mods:                           # order = merge precedence for mission files (later wins)
     - {id: 1559212036}                    # Community Framework
     - {id: 1828439124, server: true}      # loaded with -servermod
   overlays: [login-times, stamina]
   updates:
     policy: auto                  # auto | notify | manual (mods only)
     check_interval: 1h
     restart_announce: {minutes: 12, lock: 5, delay: 2, text: "MOD UPDATE!"}
   restarts:
     schedule: ["*-*-* 00/4:00"]   # systemd OnCalendar expressions
     announce: {minutes: 30, lock: 3, delay: 3}
   health: {startup_timeout: 45m, interval: 60s, retries: 5}
   restart_limit: {burst: 5, interval: 30min}
   backup:
     keep: 20
   notify: {discord: [default, admins]}
   container:
     env: {}
     mounts: ["/var/lib/GeoIP:/var/lib/GeoIP:ro"]
     memory: null                  # e.g. 24G
   hooks:
     pre_start: ["hooks/traderstocks.sh"]

Main keys:

``product``
   Which server build family the instance runs. The exact build is chosen with
   ``dzo instance upgrade`` (see :doc:`mods-and-updates`).

``map`` and ``mission_source``
   The mission folder name and where its pristine copy comes from. See
   :doc:`missions`.

``ports``
   Game, RCon and Steam query port. dzo refuses overlapping ports between
   instances.

``network``
   ``host`` (default) uses the host's network directly. ``publish`` gives the
   container its own network namespace and publishes only the game ports.

``mods``
   Workshop ids. ``server: true`` loads the mod with ``-servermod``. The list
   order decides which mod wins when two mods change the same mission file. It
   is not a game load order: DayZ orders addons by their declared dependencies.
   dzo checks those dependencies before every start and refuses to start with a
   missing one.

``updates.policy``
   What happens when a mod update is found: ``auto`` restarts the server
   gracefully within the update windows, ``notify`` only reports, ``manual``
   only records it.

``restarts.schedule``
   Maintenance restarts, as systemd calendar expressions.

``health``
   ``startup_timeout`` is how long a start may take (large mod lists load
   slowly). ``interval`` × ``retries`` is how long a hung server may stay hung
   before it is killed.

``restart_limit``
   Crash-loop brake: after ``burst`` starts within ``interval`` the server stays
   stopped and you are alerted.

serverDZ.cfg
------------

Keep your normal ``serverDZ.cfg`` in the instance directory. dzo enforces the
keys it owns (ports, ``template``, ``instanceId``, ``steamQueryPort``) and shows
a diff when your file and the enforced values differ (``dzo config diff``).
