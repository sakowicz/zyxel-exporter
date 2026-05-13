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

Each metric is published as a Home Assistant sensor under a single device named **Zyxel PoE Switch**, so it appears automatically in HA if MQTT discovery is enabled.

## Requirements

- A Zyxel PoE switch with SSH enabled and a user with **privilege level 3 or higher** so it can run `show pwr`. Tested on the **Zyxel XMG1915-10EP**, but should work on any Zyxel PoE switch that exposes SSH and the `show pwr` command.
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

- State: `zyxel/sensor/<metric_id>/state` (e.g. `zyxel/sensor/total_power/state`, `zyxel/sensor/port_1_power/state`)
- Discovery: `homeassistant/sensor/zyxel_poe/<metric_id>/config` (retained)

Discovery is re-published on connect and whenever the number of detected ports changes.

## License

MIT
