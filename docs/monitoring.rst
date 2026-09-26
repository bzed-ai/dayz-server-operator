Monitoring and notifications
============================

dzo is built to be watched from another machine: Prometheus scrapes metrics,
Icinga checks a status endpoint, and Discord gets notifications.

The exporter
------------

``dzo-exporter.service`` serves

* ``GET /metrics`` in Prometheus format,
* ``GET /status`` and ``GET /status/<instance>`` as JSON for Icinga.

It listens on ``:9464`` over plain HTTP by default. It exposes no secrets and no
player IP addresses.

.. code-block:: yaml

   exporter:
     listen: ":9464"
     tls:
       cert_file: /etc/dzo/tls/cert.pem
       key_file: /etc/dzo/tls/key.pem
       client_ca_file: null        # set for mutual TLS
     allow: ["192.0.2.0/24"]       # optional IP allow-list
     bearer_token_file: null       # optional

**Certificates are reloaded without restarts.** Replace the files (for example
from a certbot or lego deploy hook) and run
``systemctl --user reload dzo-exporter``, or just wait: changed files are picked
up automatically. Game servers are never involved, so a certificate change never
restarts a server. An invalid new certificate is rejected and the old one stays
active.

Main metrics
------------

.. list-table::
   :header-rows: 1

   * - Metric
     - Meaning
   * - ``dzo_instance_up``
     - server running and answering
   * - ``dzo_instance_health{state}``
     - ``starting``, ``healthy`` or ``unhealthy``
   * - ``dzo_instance_players``
     - players online
   * - ``dzo_instance_restarts_total{reason}``
     - restarts by ``crash``, ``health``, ``scheduled``, ``update``, ``manual``
   * - ``dzo_instance_last_render_success``
     - 0 when the last render failed
   * - ``dzo_instance_mission_drift_files``
     - managed mission files changed outside dzo
   * - ``dzo_instance_mods_pending_update``
     - mod updates waiting for a restart
   * - ``dzo_product_update_available``
     - a new server build is available (manual upgrade needed)
   * - ``dzo_steam_session_valid``
     - 0 when a Steam login is needed
   * - ``dzo_disk_free_bytes``, ``dzo_cache_bytes``
     - disk usage

Icinga
------

Install the ``dzo`` package on the Icinga host or satellite as well. It provides
monitoring plugins that follow the usual OK/WARNING/CRITICAL/UNKNOWN rules:

.. code-block:: sh

   # everything dzo knows about one instance, with players/uptime/restarts as perfdata
   dzo check remote --url https://<game host>:9464/status --instance deerisle

   # global checks
   dzo check remote --url https://<game host>:9464/status --updates
   dzo check remote --url https://<game host>:9464/status --steam
   dzo check remote --url https://<game host>:9464/status --disk

   # the game server itself, independent of dzo
   dzo check a2s <game host>:8716

Icinga 2 ``CheckCommand`` definitions and example services are shipped in
``/usr/share/doc/dzo/examples/icinga2/``.

Discord
-------

Configure a default webhook and, optionally, named webhooks with event filters
in ``/etc/dzo/config.yaml``. Each instance uses the default unless its
``notify.discord`` lists other targets (``[]`` disables notifications).

Events include mod updates, available server builds, restarts, health changes,
crash loops, failed renders, mission drift, Steam login required, and failed
jobs. Bursts are combined into one message ("12 mods updated, 3 servers
restarting").

.. code-block:: sh

   dzo notify test [--target admins]

Logs in Loki
------------

dzo does not ship logs itself. It writes them to stable places that a log agent
(Grafana Alloy, promtail, vector) can collect:

* journald: ``dzo-<name>.service`` (server console) and ``dzo-*`` jobs, as
  structured JSON with ``instance``, ``job`` and ``event`` fields;
* LogZ output in the configured ``container.logz_dir``.

``dzo loki-config`` prints a ready-made agent configuration snippet.

Operator jobs (update check, download, render, restart) can also be exported as
OpenTelemetry traces if an OTLP endpoint is configured.
