Command reference
=================

All commands run as the service user (``sudo -iu dayz``). ``dzo <command>
--help`` shows every option, and the man pages have the full reference.

Setup and Steam
---------------

.. list-table::
   :widths: 45 55

   * - ``dzo setup [--dry-run]``
     - create directories, build images, check requirements
   * - ``dzo steam login [--user <name>] [--passthrough]``
     - interactive Steam login (password and Steam Guard)
   * - ``dzo steam status``
     - account, last login, session state
   * - ``dzo steam reset-cache``
     - wipe steamcmd's working directory

Site repository
---------------

.. list-table::
   :widths: 45 55

   * - ``dzo site pull | status | commit | validate``
     - update, inspect, commit and validate the site checkout

Products and updates
--------------------

.. list-table::
   :widths: 45 55

   * - ``dzo product install <product>``
     - first download of a server build
   * - ``dzo product update <product> [--force]``
     - download a new server build (manual only)
   * - ``dzo update check [--apply]``
     - check mods (and apply per policy); report new server builds

Instances
---------

.. list-table::
   :widths: 45 55

   * - ``dzo instance create | apply | remove <name>``
     - create, (re)generate units and timers, remove
   * - ``dzo instance clone <old> <new>``
     - copy an instance's data into a new instance
   * - ``dzo instance upgrade <name> --build <id> [--wipe] [--dry-run]``
     - switch to another server build
   * - ``dzo instance mods <name> list | add | remove | move``
     - edit the mod list
   * - ``dzo instance ack-failure <name>``
     - allow starts again after a failed render or crash loop
   * - ``dzo start | stop <name>``
     - start, stop (stays stopped)
   * - ``dzo restart <name> [--minutes N --lock N --delay N --text T] [--now] [--cancel]``
     - graceful restart
   * - ``dzo status [<name>]``
     - build, mods, state, players, pending updates
   * - ``dzo logs <name> [-f]``
     - server console
   * - ``dzo shell <name>`` / ``dzo exec <name> …``
     - debug container with the same mounts
   * - ``dzo rcon <name> [command]`` / ``dzo rcon rotate <name>``
     - RCon console, new RCon password
   * - ``dzo wipe <name>``
     - wipe the world (confirmation and snapshot)

Mods
----

.. list-table::
   :widths: 45 55

   * - ``dzo mod add <id> [--instance <name>] [--server]``
     - add a workshop mod
   * - ``dzo mod remove <id> [--instance <name>]``
     - remove a mod
   * - ``dzo mod update [<id>…]``
     - check and download updates now
   * - ``dzo mod refresh <id>… | --all [--instance <name>] [--no-restart]``
     - force a fresh download
   * - ``dzo mod deps <name>``
     - dependency graph of an instance's mods
   * - ``dzo integration check <modid>``
     - fetch, normalise and validate a mod's integration files

Missions and config
-------------------

.. list-table::
   :widths: 45 55

   * - ``dzo render <name> [--dry-run] [--diff]``
     - build the mission and show what would change
   * - ``dzo mission init | update | status <name>``
     - first live mission, fetch new pristine version, state
   * - ``dzo mission rollback <name> [<time>]``
     - restore managed files from file history
   * - ``dzo mission reinit <name>``
     - recreate the live mission (confirmation and snapshot)
   * - ``dzo config diff | apply <name>``
     - serverDZ.cfg changes

Backups
-------

.. list-table::
   :widths: 45 55

   * - ``dzo backup create | list | diff | prune <name>``
     - snapshots
   * - ``dzo backup pin | unpin <id>``
     - protect a snapshot from pruning
   * - ``dzo restore <name> <id> [--path <path>]``
     - full or partial restore

Monitoring and notifications
----------------------------

.. list-table::
   :widths: 45 55

   * - ``dzo check remote --url <status url> [--instance <name>] [--updates] [--steam] [--jobs] [--disk]``
     - Icinga plugin
   * - ``dzo check a2s <host>:<query port>``
     - Icinga plugin, game server only
   * - ``dzo notify test [--target <name>]``
     - send a test notification
   * - ``dzo loki-config``
     - print a log agent configuration snippet

Web users and players
---------------------

.. list-table::
   :widths: 45 55

   * - ``dzo user list | add | disable | enable <name>``
     - web users; ``add`` prints a one-time enrolment link
   * - ``dzo user reset-password | reset-mfa | revoke-sessions <name>``
     - account recovery
   * - ``dzo player forget <steamid>``
     - erase a player's personal data

Maps
----

.. list-table::
   :widths: 45 55

   * - ``dzo map source set <map> <file>`` / ``dzo map source update <map>``
     - set a map's data PBO, retry the automatic download
   * - ``dzo map tiles status | update | export <map>``
     - map tile state, rebuild, export

Migration and development
-------------------------

.. list-table::
   :widths: 45 55

   * - ``dzo legacy convert-config --repo <dir> --ref <branch>… --out <dir>``
     - example site config from dayzdockerserver branches
   * - ``dzo test boot <name> [--server steam|<dir>] [--native|--container]``
     - boot a real server on a render result (development machines only)
