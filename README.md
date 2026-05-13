# zyxel-exporter

A small Go service that polls a Zyxel switch over SSH and exposes PoE power, system CPU/memory, and temperature metrics over **Prometheus** and (optionally) **MQTT with Home Assistant auto-discovery**.

## What it exposes

For the switch as a whole:

- Total / Consuming / Remaining PoE power (W)
- PoE Usage (%)
- Junction Temperature (°C)
- CPU Usage (%)
- Memory Total / Used (bytes) and Memory Usage (%)
- Learned MAC address count

Per-port:

- PoE power draw (W)
- Link up/down, negotiated speed (Mbps), port uptime (s)
- Tx/Rx rate (bytes/sec) and Tx/Rx link utilization (%)

## Endpoints

| Path | Purpose |
|---|---|
| `/metrics` | Prometheus scrape endpoint |
| `/health` | Liveness probe (returns `ok`) |

Both listen on `:8080` by default (override with `HTTP_LISTEN`).

## Requirements

- A Zyxel PoE switch with SSH enabled and a user with **privilege level 3 or higher** so it can run `show pwr`, `show system-information`, `show memory`, `show cpu-utilization`, `show interfaces status`, `show interfaces utilization` and `show mac-count`. Tested on the **Zyxel XMG1915-10EP**, but should work on any Zyxel PoE switch that exposes SSH and these commands.
- Prometheus (or any scraper compatible with the OpenMetrics text format).
- Optional: an MQTT broker + Home Assistant if you want HA auto-discovery.

> On the switch, create a login under **System → Logins** with **Privilege = 3** (or higher) and use those credentials in `ZYXEL_DEVICE_USERNAME` / `ZYXEL_DEVICE_PASSWORD`.

## Configuration

All configuration is via environment variables:

| Variable | Required | Default | Description |
|---|---|---|---|
| `ZYXEL_DEVICE_IP` | yes | — | Switch IP or hostname |
| `ZYXEL_DEVICE_USERNAME` | yes | — | SSH username |
| `ZYXEL_DEVICE_PASSWORD` | yes | — | SSH password |
| `CRON_SCHEDULE` | no | `* * * * *` | Polling schedule (cron syntax) |
| `HTTP_LISTEN` | no | `:8080` | HTTP listen address for `/metrics` and `/health` |
| `MQTT_BROKER_HOST` | no | — | If set, also publish to MQTT with HA discovery. If unset, MQTT is disabled. |
| `MQTT_BROKER_PORT` | no | `1883` | MQTT broker port |
| `MQTT_BROKER_USERNAME` | no | — | MQTT username |
| `MQTT_BROKER_PASSWORD` | no | — | MQTT password |

## Prometheus

Exposed metrics (prefixed `zyxel_`):

| Metric | Type | Labels | Description |
|---|---|---|---|
| `zyxel_info` | gauge | `model`, `name`, `serial`, `mac`, `fw_version`, `hw_version` | Static device info, always `1` |
| `zyxel_poe_total_watts` | gauge | — | Total PoE budget |
| `zyxel_poe_consuming_watts` | gauge | — | PoE consumed |
| `zyxel_poe_remaining_watts` | gauge | — | PoE remaining |
| `zyxel_poe_usage_percent` | gauge | — | PoE usage % (0–100) |
| `zyxel_poe_port_watts` | gauge | `port` | Per-port PoE power |
| `zyxel_junction_temperature_celsius` | gauge | — | Switch junction temperature |
| `zyxel_cpu_usage_percent` | gauge | — | CPU usage % (0–100) |
| `zyxel_memory_total_bytes` | gauge | — | Total RAM |
| `zyxel_memory_used_bytes` | gauge | — | Used RAM |
| `zyxel_memory_usage_percent` | gauge | — | RAM usage % (0–100) |
| `zyxel_port_link_up` | gauge | `port` | 1 if link is up, 0 if down |
| `zyxel_port_speed_mbps` | gauge | `port` | Negotiated speed in Mbps (0 when down) |
| `zyxel_port_uptime_seconds` | gauge | `port` | Seconds since the port last came up |
| `zyxel_port_tx_bytes_per_second` | gauge | `port` | Transmit rate (bytes/sec, sampled) |
| `zyxel_port_rx_bytes_per_second` | gauge | `port` | Receive rate (bytes/sec, sampled) |
| `zyxel_port_tx_utilization_percent` | gauge | `port` | Tx link utilization (0–100) |
| `zyxel_port_rx_utilization_percent` | gauge | `port` | Rx link utilization (0–100) |
| `zyxel_mac_count` | gauge | — | Total MAC addresses learned by the switch |

Example Prometheus scrape config:

```yaml
scrape_configs:
  - job_name: zyxel
    scrape_interval: 30s
    static_configs:
      - targets: ['zyxel-exporter:8080']
```

### Grafana dashboard

A ready-made dashboard is at [`grafana/zyxel-exporter.json`](grafana/zyxel-exporter.json). It covers every exported metric — switch info, PoE budget + per-port power, CPU/memory, junction temperature, port link status (state timeline), Tx/Rx traffic (Rx mirrored on the negative axis), link utilization, port speed and uptime.

To import: in Grafana go to **Dashboards → New → Import**, upload the JSON and pick your Prometheus datasource when prompted.

## Home Assistant (MQTT, optional)

Set `MQTT_BROKER_HOST` (and optionally username/password) to enable. On startup the exporter runs `show system-information` and uses the result to populate the HA device block per the [MQTT discovery spec](https://www.home-assistant.io/integrations/mqtt/#mqtt-discovery):

- `name` — the switch's configured System Name (falls back to `Zyxel <Model>`)
- `model` — e.g. `XMG1915-10EP`
- `manufacturer` — `Zyxel`
- `connections` — `[["mac", "<ethernet address>"]]`
- `serial_number` — switch serial
- `identifiers` — derived from the serial number, so multiple switches don't collide
- `sw_version` — ZyNOS firmware version
- `hw_version` — hardware revision
- `configuration_url` — `http://<switch ip>` (links from the HA device page to the web UI)

### MQTT topics

Topics are namespaced by a device **slug** — the switch's serial number (lowercased), or its MAC if the serial isn't readable, or `switch` as a final fallback. This means multiple instances on the same broker won't collide.

- State: `zyxel/<slug>/<metric_id>/state` — e.g. `zyxel/s252l23001041/total_power/state`, `zyxel/s252l23001041/port_1_power/state`
- Discovery: `homeassistant/sensor/zyxel_poe_<slug>/<metric_id>/config` (retained)

Discovery is re-published on (re)connect and whenever the number of detected ports changes.

## Running with Docker

A prebuilt image is published to GHCR by CI:

```
ghcr.io/sakowicz/zyxel-exporter:latest
```

Or use the included `docker-compose.yml`:

```bash
docker compose up -d
```

Edit the `environment` block in `docker-compose.yml` to match your switch. MQTT vars are commented out — uncomment them if you want HA discovery too.

## Running from source

```bash
go build -o zyxel-exporter .
ZYXEL_DEVICE_IP=192.168.1.254 \
ZYXEL_DEVICE_USERNAME=admin \
ZYXEL_DEVICE_PASSWORD=secret \
./zyxel-exporter
```

Then: `curl localhost:8080/metrics`.

## License

MIT
