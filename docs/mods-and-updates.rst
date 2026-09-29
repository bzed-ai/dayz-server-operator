.. SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
.. SPDX-License-Identifier: AGPL-3.0-or-later

Mods, updates and server upgrades
=================================

Mods update automatically. Server builds never do.

Adding and removing mods
------------------------

.. code-block:: sh

   dzo mod add <workshop id | local name> --instance deerisle [--server]
   dzo mod list [deerisle]
   dzo instance mods deerisle list
   dzo instance mods deerisle move <id> --before <other id>
   dzo mod remove <workshop id> --instance deerisle

``dzo mod add`` installs the mod first, and only then adds it to the instance's
``instance.yaml`` in the site repository (the file's comments are kept; the
change is left uncommitted for you to commit). It takes effect at the next
restart.

Before every start dzo reads the ``CfgPatches`` of all loaded mods and checks
that every required addon is present. A missing dependency stops the start with
a clear message instead of a crash loop. ``dzo mod deps deerisle`` shows the
dependency graph.

Reading ``config.bin``
~~~~~~~~~~~~~~~~~~~~~~

Mods ship their config as a rapified (binary) ``config.bin``, usually inside
an LZSS-compressed PBO entry. dzo decodes both itself, so no extra tools are
needed: it looks for ``config.bin`` at the root of each addon PBO (falling back
to a plain ``config.cpp``) and reads only the ``CfgPatches`` classes and their
``requiredAddons``. The diagnostic commands accept a PBO, a bare
``config.bin`` or a ``config.cpp``::

   dzo mod cfgpatches @MyMod/addons/mymod.pbo
   dzo mod deps @MyMod/addons/mymod.pbo --provides DZ_Data,DZ_Scripts --order

Corrupt or truncated files produce an error naming the problem rather than a
guess. The decoder has been checked against synthetic files only; report a mod
whose ``config.bin`` it cannot read.

Automatic mod updates
---------------------

``dzo-update-check.timer`` runs every hour. For each instance whose
``check_interval`` is due, dzo asks Steam which mods changed, downloads them into
new generations while the servers keep running, and then acts on the instance's
``updates.policy``:

``auto``
   graceful restart with the ``restart_announce`` countdown, a snapshot, the
   switch to the new mod generations and a start.
``notify``
   record and notify only.
``manual``
   record only.

To avoid a restart for every single mod push, updates can be batched:

.. code-block:: yaml

   updates:
     policy: auto
     window: ["06:00-10:00", "14:00-16:00"]   # only restart in these windows
     quiet_hours: ["18:00-24:00"]            # never restart for updates here
     min_restart_interval: 4h
     batch_delay: 20m                        # wait for more updates after the first
     apply_with_scheduled_restart: true      # use a maintenance restart if one is near
     max_delay: 12h

All pending mod updates of an instance are applied in one restart.

Check by hand:

.. code-block:: sh

   dzo update check            # show what is pending
   dzo update check --apply    # download and apply according to the policies

Installing mods and server builds
---------------------------------

Everything dzo downloads is stored as an immutable *generation*: a directory
that is never changed after it appears, so a running server never sees files
move under it. A new version is a new generation, and ``current`` is switched
only after the new one is complete and validated. Re-running a command never
touches an existing generation.

.. code-block:: sh

   dzo product install dayz-stable      # first download of the server build
   dzo mod update [deerisle]            # new mod versions, for one instance or all
   dzo mod list [deerisle]              # what is installed

Workshop mods are named by Steam's ``time_updated`` (``<time_updated>`` or, after
a forced refresh, ``<time_updated>-r<n>``) and the server build by its Steam
build id. Steam downloads run one at a time: dzo holds a lock, and the Steam
session from ``dzo steam login`` must be valid (``steam.account`` in
``config.yaml`` names the account).

A downloaded mod is checked before it becomes a generation: ``meta.cpp`` must be
there and every PBO under ``addons/`` must parse. If a mod fails the check, the
other mods of the same run are still installed, and the command exits with an
error. Installing does not restart anything; running servers keep the
generation they were started with.

**Local servermods** (``{local: <name>, server: true}``) are imported the same
way, from ``localmods/<name>/`` in the site repository, a host directory, or a
release archive (``local_mods`` in ``site.yaml``, see :doc:`resolved-instance`).
The generation is named after a sha256 over the mod's files, so unchanged
content is never imported twice. Only regular files are accepted: a symlink is
an error, and archives with absolute paths, ``..``, links or device files are
rejected. An archive is checked against its sha256 before it is unpacked. The
mod root is the directory holding ``addons/`` (directly, or in a single
``@<name>`` directory); a ``keys/`` directory is ignored. Every PBO of a local
mod must carry a ``prefix`` header, because a PBO without one loads without an
error but none of its scripts run.

Broken or corrupted mod downloads
---------------------------------

There is no rollback to an older mod version: if a correctly downloaded mod is
broken, the mod needs a fix, or you remove it from the instance.

What dzo does offer is a **forced re-download**. Use it when a download is
incomplete or damaged, or when Steam's own cache serves a bad copy:

.. code-block:: sh

   dzo mod refresh <id> [<id>…] [--instance deerisle] --force
   dzo mod refresh --all --instance deerisle --force

This deletes steamcmd's cached state for the mod, downloads it again even if
Steam reports no change, checks the result (``meta.cpp`` present, every PBO
readable) and stores it as a new generation. Servers switch to it at their next
restart. ``--force`` is required, to make it clear that this
downloads regardless of Steam's version. The web interface has the same action
per mod.

As a last resort, ``dzo steam reset-cache`` wipes steamcmd's working directory
after confirmation. The next download fills it again.

Server upgrades (manual only)
-----------------------------

dzo checks for new server builds but **never downloads or applies one on its
own**. A new DayZ version often needs a wipe, mod updates or config changes, so
you decide when and how each server moves.

When a new build is available you get a Discord message, a warning from
``dzo check remote --updates``, and a note in ``dzo status``. Then:

#. Download the build. Running servers are not affected:

   .. code-block:: sh

      dzo product update dayz-stable

#. Prepare what the new version needs: mod updates, new mods, config changes in
   the site repository, and whether to wipe.

#. Look at the result before switching:

   .. code-block:: sh

      dzo instance upgrade deerisle --build <buildid> --dry-run

   This shows the render plan and checks the mod dependencies against the new
   build.

#. Switch the instance:

   .. code-block:: sh

      dzo instance upgrade deerisle --build <buildid> [--wipe]

   The instance is announced, stopped, snapshotted, optionally wiped, switched
   to the new build, rendered and started. Upgrade each instance when it is
   ready; nothing forces all servers of a product to move at once.

``dzo product update <product> --force`` validates and re-copies the current
build, the server-side equivalent of ``dzo mod refresh``.

Steam login
-----------

Downloads use the session created by ``dzo steam login``. The password is not
stored by default. When Steam ends the session:

* downloads pause (nothing fails), and running servers are not affected;
* Discord, ``/metrics`` (``dzo_steam_session_valid 0``) and
  ``dzo check remote --steam`` report it;
* after ``dzo steam login`` (on the host or in the web interface) paused jobs
  resume automatically.

dzo never retries a failed password or code automatically, so it cannot lock
the account. The hourly check also tests the session, so an expired login is
noticed before an update needs it.
