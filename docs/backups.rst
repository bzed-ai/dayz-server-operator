.. SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
.. SPDX-License-Identifier: AGPL-3.0-or-later

Backups and restores
====================

Every instance lives in its own btrfs subvolume. A backup is a read-only btrfs
snapshot of that subvolume: it takes milliseconds and costs almost no space
until files change. Snapshots are taken with the server stopped, so they are
consistent.

When snapshots are taken
------------------------

.. list-table::
   :header-rows: 1

   * - Reason
     - When
     - Default
   * - ``update``
     - before ``dzo instance upgrade`` switches to a new server build
     - on
   * - ``mod_update``
     - before new mod versions are applied, including automatic updates
     - on
   * - ``mission_update``
     - before a new pristine mission is applied
     - on
   * - ``config_change``
     - before site repository changes are applied
     - on
   * - ``render``
     - before every start, including crash restarts
     - off
   * - ``scheduled``
     - periodically while running (``backup.schedule``)
     - off
   * - ``manual``
     - ``dzo backup create <name>``
     - –
   * - ``destructive``
     - before ``mission reinit``, ``wipe`` and restores
     - always

If a snapshot fails, the change is not applied, the server starts unchanged and
you are alerted.

Retention
---------

.. code-block:: yaml

   backup:
     before: [update, mod_update, mission_update, config_change]
     schedule: null            # e.g. "*-*-* 00/6:00"
     keep: 20
     keep_by_reason: {mod_update: 8, update: 10, scheduled: 12}
     max_age: 30d
     min_keep: 3               # never fewer, regardless of age
     min_free_bytes: 50GiB     # prune the oldest first to keep this free

Old snapshots are pruned after each successful snapshot, by the daily
``dzo-backup-prune.timer``, and on demand:

.. code-block:: sh

   dzo backup prune [<name>] --dry-run
   dzo backup pin <id>        # never pruned automatically
   dzo backup unpin <id>

dzo only deletes snapshots it has recorded. Unknown directories in the snapshot
folder are reported, not deleted.

Listing and comparing
---------------------

.. code-block:: sh

   dzo backup list [<name>]
   dzo backup diff <id> [<id2>|live]

Snapshots are plain read-only directories, so you can also browse them directly
or download files in the web interface.

Restoring
---------

Full restore:

.. code-block:: sh

   dzo restore deerisle <id>

The instance is stopped, a safety snapshot is taken, the current data is moved
aside (and kept as a snapshot), the backup becomes the live instance, and the
server starts.

Restore only part of an instance, for example the world or one mod's data:

.. code-block:: sh

   dzo restore deerisle <id> --path mpmissions/empty.deerisle/storage_1

The database (players, bans, schedules, users) is backed up separately every day
and before database migrations.

Offsite copies
--------------

Snapshots stay on the same filesystem, so they do not protect against disk
loss. Add a ``post_backup`` hook that copies new snapshots elsewhere, for
example with restic or borg; example hooks are shipped in
``/usr/share/doc/dzo/examples/backup-hooks/``.
