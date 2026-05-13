# zyxel-poe-to-mqtt

A small Go service that polls a Zyxel PoE switch over SSH and publishes power metrics to MQTT, with Home Assistant auto-discovery.

## What it exposes

For the switch as a whole:

- Total Power (W)
- Consuming Power (W)
- Remaining Power (W)
- PoE Usage (%)
- Junction Temperature (°C)

Plus per-port power draw in watts for every PoE port reported by `show pwr`.

Each metric is published as a Home Assistant sensor under a single MQTT device, so it appears automatically in HA if MQTT discovery is enabled.

On startup the service also runs `show system-information` and uses the result to populate the HA device block per the [MQTT discovery spec](https://www.home-assistant.io/integrations/mqtt/#mqtt-discovery):

- `name` — the switch's configured System Name (falls back to `Zyxel <Model>`)
- `model` — e.g. `XMG1915-10EP`
- `manufacturer` — `Zyxel`
- `connections` — `[["mac", "<ethernet address>"]]`
- `serial_number` — switch serial
- `identifiers` — derived from the serial number, so multiple switches don't collide
- `sw_version` — ZyNOS firmware version
- `hw_version` — hardware revision
- `configuration_url` — `http://<switch ip>` (links from the HA device page to the web UI)

## Requirements

- A Zyxel PoE switch with SSH enabled and a user with **privilege level 3 or higher** so it can run `show pwr` and `show system-information`. Tested on the **Zyxel XMG1915-10EP**, but should work on any Zyxel PoE switch that exposes SSH and those two commands.
- An MQTT broker reachable from the container
- Home Assistant with the MQTT integration (optional — discovery is published to the `homeassistant/` prefix)

> On the switch, create a login under **System → Logins** with **Privilege = 3** (or higher) and use those credentials in `ZYXEL_DEVICE_USERNAME` / `ZYXEL_DEVICE_PASSWORD`.

## Configuration

All configuration is via environment variables:

| Variable | Required | Default | Description |
|---|---|---|---|
| `ZYXEL_DEVICE_IP` | yes | — | Switch IP or hostname |
| `ZYXEL_DEVICE_USERNAME` | yes | — | SSH username |
| `ZYXEL_DEVICE_PASSWORD` | yes | — | SSH password |
| `MQTT_BROKER_HOST` | yes | — | MQTT broker hostname |
| `MQTT_BROKER_PORT` | no | `1883` | MQTT broker port |
| `MQTT_BROKER_USERNAME` | no | — | MQTT username |
| `MQTT_BROKER_PASSWORD` | no | — | MQTT password |
| `CRON_SCHEDULE` | no | `* * * * *` | Polling schedule (cron syntax) |

A `/health` endpoint is exposed on port `8080`.

## Running with Docker

A prebuilt image is published to GHCR by CI:

```
ghcr.io/sakowicz/zyxel-poe-to-mqtt:latest
```

Or use the included `docker-compose.yml`:

```bash
docker compose up -d
```

Edit the `environment` block in `docker-compose.yml` to match your switch and broker.

## Running from source

```bash
go build -o zyxel-to-mqtt .
ZYXEL_DEVICE_IP=192.168.1.254 \
ZYXEL_DEVICE_USERNAME=admin \
ZYXEL_DEVICE_PASSWORD=secret \
MQTT_BROKER_HOST=192.168.1.10 \
./zyxel-to-mqtt
```

## MQTT topics

Topics are namespaced by a device **slug** — the switch's serial number (lowercased), or its MAC if the serial isn't readable, or `switch` as a final fallback. This means multiple instances on the same broker won't collide.

- State: `zyxel/<slug>/<metric_id>/state` — e.g. `zyxel/s252l23001041/total_power/state`, `zyxel/s252l23001041/port_1_power/state`
- Discovery: `homeassistant/sensor/zyxel_poe_<slug>/<metric_id>/config` (retained)

Discovery is re-published on (re)connect and whenever the number of detected ports changes.

## License

MIT
