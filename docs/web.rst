.. SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
.. SPDX-License-Identifier: AGPL-3.0-or-later

Web interface
=============

``dzo web`` serves the web interface. It is a thin layer over the :doc:`API
<api>`: every button is one API call, so what you can do in the browser you can
also do with ``dzo player``, ``dzo vehicle`` or a script.

The web interface only ever talks to installations through their API. It can
run on the same host as ``dzo serve`` (the default), or on another host, and
one web interface can show **several dzo installations** side by side.

.. code-block:: sh

   sudo -iu dayz dzo token create web --role operator --delegate > /var/lib/dzo/secrets/web.token
   sudo -iu dayz dzo serve &      # the API (normally a systemd unit)
   sudo -iu dayz dzo web          # http://127.0.0.1:8081

Several installations
---------------------

List one backend per installation in ``/etc/dzo/config.yaml``. Each needs an API
token created *on that installation*:

.. code-block:: yaml

   web:
     backends:
       - {name: eu, url: "https://dzo-eu.example.invalid", token_file: /var/lib/dzo/secrets/eu.token}
       - {name: us, url: "https://dzo-us.example.invalid", token_file: /var/lib/dzo/secrets/us.token}

The dashboard shows every installation with its instances. An installation that
is down is shown with its error; the others keep working.

Access control
--------------

.. warning::

   The web interface has **no login of its own yet** (no users, passwords or
   second factor). Anyone who can reach it acts with the role of the backend
   token. Keep it on localhost, or put it behind a reverse proxy that
   authenticates your users (with multi-factor login) and terminates HTTPS.
   dzo refuses to listen on a non-local address unless
   ``web.allow_insecure_http`` is set.

Set ``web.user_header`` to the header your proxy fills with the signed-in user
(for example ``X-Forwarded-User``). That name goes into the audit log next to
each action. Only use this behind a proxy that strips the header from incoming
requests.

What the interface does about browser attacks: every change needs a per-session
token that the pages hand to htmx (and a same-origin check), responses carry a
strict content security policy without inline scripts, and pages cannot be
framed.

Roles
-----

The role of a token decides what the interface may do. Without a permission the
action fails with a clear message.

.. list-table::
   :header-rows: 1

   * - Role
     - May
   * - ``viewer``
     - see instances, players, vehicles, events and public map markers
   * - ``moderator``
     - also send direct messages to players
   * - ``operator``
     - also broadcast, teleport, spawn items, repair and delete vehicles
   * - ``admin``
     - everything, including the audit log and markers that are admin-only

Pages
-----

Dashboard
   All installations and their instances, with the dzo-admin connection state
   and player and vehicle counts.

Instance
   Broadcast message, the player list (refreshed every 5 seconds) with a panel of
   actions per player (message, teleport to coordinates or to another player,
   give item with a search over the server's item classes), and the vehicle list
   with repair and delete.

Live map
   Players, vehicles, active events and marker layers on a map that updates as
   the server reports. Click the map to teleport a player there. See
   :doc:`admin-map`.

Audit log
   Who did what, when, from where (web, CLI or API), and the result. Needs the
   ``admin`` role.

Documentation
   This documentation is served under ``/docs/`` without login.

Not available yet
-----------------

These are planned and are not part of the web interface today: users with
passwords and TOTP, single sign-on, the player database (history, sessions,
Steam profiles, GeoIP), kicks and bans, restart timers and scheduled
broadcasts, a map background, and editors for server configuration.
