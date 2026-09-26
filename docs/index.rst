dzo — DayZ server operator
==========================

dzo runs and maintains DayZ dedicated servers on a Debian host. It installs the
server and workshop mods, prepares each server's mission, runs every server as a
rootless podman container managed by systemd, restarts servers gracefully, keeps
mods up to date, takes btrfs snapshots before changes, and exports monitoring
data. An optional web interface adds player management, moderation and a live
admin map.

.. note::

   dzo is under active development. These pages describe how dzo works and is
   meant to be used. Commands and options may still change before the first
   release.

Where to start
--------------

* New to dzo: read :doc:`overview`, then follow :doc:`installation` and
  :doc:`quickstart`.
* Coming from ``dayzdockerserver``: see :doc:`migration`.
* Something is broken: see :doc:`troubleshooting`.

.. toctree::
   :maxdepth: 2
   :caption: Getting started

   overview
   installation
   quickstart
   migration

.. toctree::
   :maxdepth: 2
   :caption: Administration

   configuration
   missions
   mods-and-updates
   operations
   backups
   monitoring
   troubleshooting

.. toctree::
   :maxdepth: 2
   :caption: Web interface

   web
   admin-map

.. toctree::
   :maxdepth: 2
   :caption: Reference

   cli
   development
