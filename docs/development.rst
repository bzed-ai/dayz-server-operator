.. SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
.. SPDX-License-Identifier: AGPL-3.0-or-later

Development
===========

Principles
----------

dzo is written the lazy way: lazy meaning efficient, not careless.

* **Build what is needed now.** No features, options or abstractions "for
  later". An interface with one implementation, a factory for one product or a
  setting for a value that never changes is not added.
* **Reuse before writing.** In this order: code that already exists in dzo, the
  Go standard library, a feature of the platform (systemd, podman, btrfs, the
  database), an existing well-maintained library, and only then new code.
* **Boring over clever.** Short, obvious code that someone can debug at 3 am.
* **Never lazy about safety.** Input validation, error handling that prevents
  data loss, security measures and tests are not simplified away.
* A deliberate shortcut with a known limit gets a ``ponytail:`` comment that
  names the limit and the upgrade path.

Building
--------

dzo is a single static Go binary built with the current upstream Go toolchain.
Modules are vendored, so builds work offline.

.. code-block:: sh

   make build      # static binary
   make test       # tests with race detector and coverage gate
   make lint       # includes reuse lint (licence headers)
   make licenses   # dependency licences must be AGPL-compatible
   make docs       # this documentation (needs python3-sphinx)
   make deb        # Debian package

Tests
-----

The test suite needs no Steam account and no DayZ server: steamcmd, systemd,
podman, RCon and the Steam API are replaced by fakes and recorded fixtures.
Coverage must stay at 85 % or more; CI fails below that.

Boot test with a real server
----------------------------

Unit tests cannot prove that DayZ accepts what dzo renders. ``dzo test boot``
starts a real ``DayZServer`` headless on a render result and checks the logs.

.. warning::

   Only on a development machine or a dedicated test machine. ``dzo test boot``
   refuses to run on a host where dzo manages instances.

.. code-block:: sh

   dzo test boot deerisle --server steam --native
   dzo test boot deerisle --container --keep-tree
   dzo test boot --vanilla            # refresh the vanilla baseline

``--server steam`` uses the "DayZ Server" tool installed by your Steam client
(Library → Tools). dzo finds it through Steam's library files and never writes
into Steam's directories. Mods come from your client's workshop folder
(``--mods-from steam-client``) or from a directory of ``@Mod`` folders.

The test builds a throwaway server tree, starts the server on free ports with a
random password, waits until it answers Steam queries, stops it, and checks:

* the server did not crash and became ready in time;
* no script compile errors (``SCRIPT (E)`` in ``script_*.log``);
* every mod's scripts were loaded: the number of loaded script files per module
  must rise over the vanilla baseline (an absolute mod path or a PBO without a
  prefix silently loads nothing);
* no known Central Economy or mission errors in the RPT and ``error.log``;
* expected log lines, and the dzo-admin handshake if the mod is loaded.

Logs are kept in ``--out`` (default ``./boottest-<instance>-<time>/``).

Contributing
------------

* Code and documentation change together: a change in behaviour updates the
  matching page here.
* The documentation is plain reStructuredText in ``docs/``, built with Sphinx.
  ``make docs`` must build without warnings.
* dzo is licensed under the AGPL-3.0-or-later. The repository follows the
  `REUSE <https://reuse.software/>`_ specification: every new file starts with
  SPDX headers in the file's comment syntax, for example in Go:

  .. code-block:: go

     // SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
     // SPDX-License-Identifier: AGPL-3.0-or-later

  Files that cannot carry comments (images, test fixtures) get a
  ``<file>.license`` file next to them or an entry in ``REUSE.toml``.
  ``make lint`` runs ``reuse lint`` and fails on files without licence
  information.
