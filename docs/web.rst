.. SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
.. SPDX-License-Identifier: AGPL-3.0-or-later

Web interface
=============

``dzo serve`` (the ``dzo-web.service`` unit) adds a web interface and a JSON
API on top of everything the command line does. Every action in the interface
is an API call and is recorded in the audit log.

It listens on localhost by default. Put it behind a reverse proxy with HTTPS,
or give it its own certificate (reloaded without restarts, like the exporter).
Plain HTTP on a non-local address is refused.

This documentation is served by the web interface under ``/docs/``, without
login.

Signing in
----------

Every user signs in with a **password and a TOTP code** from an authenticator
app (any app that supports standard 6-digit, 30-second codes).

First administrator
~~~~~~~~~~~~~~~~~~~

There is no default password. Create the first administrator on the host:

.. code-block:: sh

   sudo -iu dayz dzo user add alice --role admin

This prints a one-time enrolment link (valid for 24 hours). Open it, then

#. choose a password (at least 12 characters),
#. scan the QR code with your authenticator app,
#. confirm with a code,
#. store the 10 recovery codes somewhere safe. They are shown only once.

Further users are invited the same way from the user administration page.

Sessions and security
~~~~~~~~~~~~~~~~~~~~~

* A session ends after 30 minutes without activity and after 12 hours at the
  latest. You can see and end your sessions on your profile page.
* Sensitive actions ask for a fresh TOTP code: user and role changes, global or
  country bans, actions on all servers, wipes and restores, secret changes, and
  API tokens.
* After 10 failed attempts an account is locked for 15 minutes. Lockouts,
  recovery-code use and MFA resets are reported to the admin Discord channel.
* Lost your phone? Use a recovery code, or ask an administrator to run
  ``dzo user reset-mfa <name>`` on the host.

Single sign-on
~~~~~~~~~~~~~~

OIDC login is optional. dzo accepts the identity provider's MFA only if the
login token says MFA was used; otherwise it asks for its own TOTP code as well.
Roles can be mapped from a groups claim.

API tokens
~~~~~~~~~~

Bots and scripts use API tokens instead of logins. A token has a name, a set of
permissions and servers, and an expiry date. It is shown once, and can never
manage users, roles or other tokens.

Roles
-----

.. list-table::
   :header-rows: 1

   * - Role
     - Typical use
   * - ``viewer``
     - see status, players and the map
   * - ``moderator``
     - kick, ban, message players
   * - ``operator``
     - restarts, schedules, broadcasts, mods, configuration
   * - ``admin``
     - everything, including users

Roles are granted globally or per server, for example "moderator on deerisle
only". Individual permissions such as ``player.ban.global``,
``player.spawn_item`` or ``pii.view`` (seeing IP addresses) refine them.

Players
-------

Players are always identified by their Steam ID, never by name. For each player
dzo keeps first and last login, playtime per server, name history, sessions and
the country (from GeoIP). The player page also shows Steam profile information
and VAC or game bans.

IP addresses are personal data: only users with ``pii.view`` see them, they are
kept for a limited time (then only the country remains), and
``dzo player forget <steamid>`` erases a player's data (except active bans).

Moderation
----------

Kick
   From the player list or page, with a reason.

Ban
   By Steam ID, BattlEye GUID, IP address or range, country or network (ASN).
   For one, several or all servers; permanent or temporary; with reason, public
   message and notes. Country and network bans can have exceptions for
   individual Steam IDs.

   Bans are checked when a player joins. Steam ID and GUID bans are also written
   to the server's own ban lists, so they hold even while dzo is down.

Whitelist mode
   Only listed players may join. Useful for events and test servers.

Watchlist and notes
   Get notified when a watched player joins any server. Notes and evidence
   links are visible to moderators.

Restarts, broadcasts and schedules
----------------------------------

* "Restart now" with a countdown, or immediately.
* One-off restarts ("today at 18:30 with a 15 minute countdown"), and "restart
  when empty".
* Pending restarts can be postponed or cancelled.
* Broadcasts: send now to one, several or all servers, or on a schedule, with
  placeholders such as ``{next_restart_in}`` and ``{players}``. Direct messages
  to single players.

Scheduled items become systemd timers, so they also run when the web interface
is not running.

Item spawning
-------------

With the ``dzo-admin`` server mod (see :doc:`admin-map`) you can give items to
players: search by class or display name, filter by category, choose quantity
and condition, or use a saved kit. The list of items comes from the running
server and the server's Central Economy files, so modded items appear
automatically. Spawns are limited by permissions and an allow/deny list per
server, and every spawn is logged.
