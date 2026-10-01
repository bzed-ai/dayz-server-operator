.. SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
.. SPDX-License-Identifier: AGPL-3.0-or-later

Troubleshooting
===============

Short runbooks for the situations that come up most. Each one starts with the
symptom you see.

A server does not start: "render failed"
-----------------------------------------

**Symptom:** ``dzo start`` fails, Discord reports a failed render, the live
mission is unchanged.

#. Show the problem: ``dzo instance render <name> --dry-run``. It names the file and the
   error (invalid XML, a missing referenced file).
#. Fix it in the site repository, commit, then ``dzo site pull``.
#. ``dzo start <name>``.

If the instance is marked ``failed-render`` after a crash restart, starting is
blocked until its inputs change. After fixing the cause, or to retry as is:
``dzo instance ack-failure <name>``.

A server is in a crash loop
---------------------------

**Symptom:** Discord reports a crash loop, the unit is ``failed``, Icinga is
critical.

#. ``dzo logs <name>`` and the crash summary in the journal: look at the
   ``script*.log`` and ``*.RPT`` tails.
#. Typical causes: a broken mod update, a script error in a mod, a mod that
   needs a newer server build.
#. A damaged download: ``dzo mod refresh <id> --instance <name>``.
#. A broken mod: remove it (``dzo mod remove <id> --instance <name>``) until it
   is fixed, or restore a snapshot from before the update if the world data was
   affected (:doc:`backups`).
#. ``dzo instance ack-failure <name>``, then ``dzo start <name>``.

"Steam login required"
----------------------

**Symptom:** downloads are paused, ``dzo_steam_session_valid`` is 0.

Running servers are not affected. Log in again, on the host with
``dzo steam login`` or in the web interface, and confirm Steam Guard. Paused
jobs resume on their own. If Steam reports a rate limit, wait until the time
shown by ``dzo steam status``; dzo never retries on its own.

A mod download is broken or Steam keeps serving a bad copy
----------------------------------------------------------

.. code-block:: sh

   dzo mod refresh <id> [--instance <name>]

If that does not help, ``dzo steam reset-cache`` wipes steamcmd's working
directory, and the next download starts from scratch.

A new DayZ version is out
-------------------------

Servers keep running on the old build and players with the new client cannot
join. Follow the upgrade steps in :doc:`mods-and-updates`: download the build,
prepare mods and config, ``dzo instance upgrade <name> --build <id> --dry-run``,
then upgrade.

"Drift detected"
----------------

**Symptom:** Discord reports that managed mission files changed.

Something other than dzo changed a file that dzo manages, and dzo overwrote it
with its version after saving a copy in ``filehistory/``.

* If the change was a mistake, nothing to do.
* If a mod writes this file on purpose, add it to ``mission.unmanaged`` in
  ``instance.yaml``.
* If you want to keep a manual change, move it into an overlay. Recover the
  changed file with ``dzo mission rollback <name>``.

Snapshots fill the disk
-----------------------

``dzo backup list`` shows sizes and expiry. Tighten ``backup.keep`` or
``max_age``, set ``min_free_bytes``, and run ``dzo backup prune --dry-run``, then
without ``--dry-run``. Pinned snapshots are never pruned automatically.

Locked out of the web interface
-------------------------------

On the host, as the service user:

.. code-block:: sh

   dzo user list
   dzo user enable <name>          # also lifts a temporary lockout
   dzo user reset-mfa <name>       # prints a new enrolment link
   dzo user reset-password <name>
