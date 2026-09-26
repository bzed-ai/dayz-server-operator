Installation
============

Requirements
------------

* Debian 13 (trixie), x86_64, with podman 5.4 and systemd 257 from Debian.
* Linux kernel 6.12 or newer (the trixie kernel, or a trixie-backports kernel).
* A **btrfs** filesystem for the instance data and the snapshots. Both must be on
  the same btrfs filesystem. The download cache can live anywhere, but on btrfs
  copies are nearly free.
* A Steam account that owns DayZ. The DayZ server cannot be downloaded
  anonymously.
* Enough disk space: about 3 GB per server build, plus mods, plus snapshots.

Recommended, not required:

* Mount the btrfs filesystem with ``user_subvol_rm_allowed``. dzo can then delete
  old snapshots instantly. Without it, deletion still works but takes longer.

Install the package
-------------------

dzo is shipped as a Debian package, ``dzo``. It pulls in ``podman``, ``passt``,
``git``, ``uidmap`` and the web libraries from Debian. steamcmd is not installed
on the host: it runs in a container image that ``dzo setup`` builds.

.. code-block:: sh

   apt install ./dzo_<version>_amd64.deb

The package

* creates the system user ``dayz`` with the home directory ``/var/lib/dzo``,
* assigns subordinate uid/gid ranges to it (needed for rootless podman),
* enables lingering, so the user's services run without a login,
* installs the documentation, which the web interface serves under ``/docs/``
  (also readable offline in ``/usr/share/doc/dzo/html/``).

It does **not** start any game server.

Choose the data locations
-------------------------

All paths are configured in ``/etc/dzo/config.yaml``. The defaults put
everything under ``/var/lib/dzo``. To use another location, for example
``/srv/dayz``:

.. code-block:: yaml

   paths:
     data: /srv/dayz
     instances: ${data}/instances    # must be btrfs
     snapshots: ${data}/snapshots    # same btrfs filesystem as instances
     cache: ${data}/cache
     secrets: ${data}/secrets
     db: ${data}/db

Run the setup
-------------

Run the setup as the service user. It creates the directories with the right
permissions, builds the container images, clones the site repository and checks
the requirements (btrfs, same filesystem for instances
and snapshots, free space, podman features).

.. code-block:: sh

   sudo -iu dayz dzo setup --dry-run   # show what would be done
   sudo -iu dayz dzo setup

``dzo setup`` reports what is missing, for example the ``user_subvol_rm_allowed``
mount option.

Log in to Steam
---------------

Downloads need a Steam login. It is interactive: you type the password and
confirm Steam Guard (a code by e-mail or from the mobile app, or a confirmation
in the app).

.. code-block:: sh

   sudo -iu dayz dzo steam login --user <steam account>
   sudo -iu dayz dzo steam status

dzo keeps the Steam session, not the password. When Steam ends the session
(password change, new device check, long inactivity), downloads pause and dzo
notifies you. Log in again with the same command. See :doc:`mods-and-updates`.

Next steps
----------

Continue with :doc:`quickstart`.
