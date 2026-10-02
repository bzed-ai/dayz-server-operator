.. SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
.. SPDX-License-Identifier: AGPL-3.0-or-later

Quick start: the first server
=============================

This walk-through creates one vanilla Chernarus server. All commands run as the
service user (``sudo -iu dayz``).

1. Point dzo at your site repository
------------------------------------

Create an empty git repository for your configuration (or use the example from
:doc:`migration`) and add it to ``/etc/dzo/config.yaml``:

.. code-block:: yaml

   site:
     url: git@<git host>:<you>/dayz-site.git
     branch: main
     commit: true      # commit changes dzo makes (mod add, web edits)
     push: false

.. code-block:: sh

   dzo site pull

2. Install the server build
---------------------------

.. code-block:: sh

   dzo product install dayz-stable

3. Describe the instance
------------------------

Create ``instances/chernarus/instance.yaml`` in the site repository:

.. code-block:: yaml

   name: chernarus
   product: dayz-stable
   map: dayzOffline.chernarusplus
   mission_source:
     preset: vanilla            # Bohemia's Central Economy repository
   ports: {game: 2302, rcon: 2303, query: 27016}
   mods: []
   restarts:
     schedule: ["*-*-* 00/4:00"]   # every 4 hours

Add a ``serverDZ.cfg`` next to it (your usual server settings; dzo sets the
mission ``template`` and the Steam query port itself), commit and pull:

.. code-block:: sh

   dzo site pull
   dzo site validate

4. Create and start it
----------------------

.. code-block:: sh

   dzo instance create chernarus    # subvolume, pristine mission, first live mission
   dzo instance render chernarus --dry-run
   dzo instance apply chernarus     # generate units and timers
   dzo start chernarus
   dzo status chernarus

``dzo start`` returns once the server answers Steam queries. Follow the console
with ``dzo logs chernarus -f``.

5. Add a mod
------------

.. code-block:: sh

   dzo mod add 1559212036 --instance chernarus     # Community Framework
   dzo restart chernarus --minutes 5

The restart announces itself in game, locks the server, kicks the remaining
players and restarts. See :doc:`operations`.

What next
---------

* Mod integrations and mission overlays: :doc:`missions`
* Automatic mod updates and server upgrades: :doc:`mods-and-updates`
* Snapshots and restores: :doc:`backups`
* Prometheus, Icinga and Discord: :doc:`monitoring`
