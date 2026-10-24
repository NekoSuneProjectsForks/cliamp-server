# NekoSune Radio Dashboard

This fork includes a public homepage at `/`, a private management UI at `/admin`, and a sanitized public API at `/api/public/radio`.

## Configuration

Set a strong TOML `[admin] password`; this is required for control and upload endpoints.

```toml
[server]
host = "0.0.0.0"
port = 8000

[admin]
password = "REPLACE_WITH_LONG_RANDOM_SECRET"

[stats]
db_path = "/data/stats.db"

[stations.neffex]
name = "Neffex Station"
path = "/music/neffex"
shuffle = true

[stations.nekoryza]
name = "Nekoryza Music Station"
path = "/music/nekoryza"
shuffle = true
```

Docker must mount the music library **read-write** for uploads, and ensure the non-root runtime user can write those directories:

```yaml
services:
  cliamp-server:
    build: .
    ports:
      - "8000:8000"
    volumes:
      - ./config.toml:/config/cliamp-server/config.toml:ro
      - ./music:/music
      - ./data:/data
```

Open `http://localhost:8000/` for the public page and `http://localhost:8000/admin` for the admin page.

## Functionality

- Public station cards, HTML5 playback, listener counts, current artist/title, and live updates every 10 seconds
- Album-cover discovery via the free iTunes Search API, with fallback artwork if unavailable
- Optional aggregated country analytics from the existing public `/statistics` endpoint (requires statistics and GeoIP data)
- Password-protected admin bearer-token requests to stop accepting new listeners per station, resume a station, and upload a music file to that station's configured path

## Current limitations

- The switch prevents **new** connections; it does not disconnect existing streaming listeners or stop the background encoder.
- New music uploads do not enter the currently running in-memory playlist until a server restart. Run `docker compose restart cliamp-server` after uploading.
- Artwork is fetched by the visitor's browser from iTunes and may not match each track. No cover images are saved on the server.
- Authentication uses the existing admin password, held in browser sessionStorage only; serve the site over HTTPS and use a long unique password.
- The global `/status` JSON endpoint is unchanged; the public homepage uses `/api/public/radio` specifically to avoid exposing listener IP addresses.
- Start/stop changes are not persisted after container restart.

Avoid making `/status` public when listener IP addresses are present.

## Listener privacy and moderation

- Public `/api/public/radio` and legacy `/status` / `/<id>/status` expose station counts and track metadata without listener IP addresses. The previous `listener_details` key is an empty array for compatibility.
- `GET /api/admin/listeners` exposes connected IPs and session information only after Bearer authentication using `[admin] password`.
- `GET /api/admin/bans` lists blocked IPs. `POST /api/admin/bans` accepts JSON `{"ip":"203.0.113.42","banned":true}` or `false` to unban.
- Bans reject new stream connections and end existing stream loops; the transport can take a moment to close if blocked on a write.
- In Docker set `CLIAMP_BAN_FILE=/data/banned-ips.json` and mount `./data:/data` read-write, granting the nonroot container user write access to persist bans between restarts. Otherwise the ban list defaults to an ephemeral temp directory.
- IP bans apply to the address seen by the server. Only configure trusted reverse-proxy forwarding; spoofed X-Forwarded-For headers can undermine IP bans.
- IP addresses may be shared by multiple listeners (NAT/VPN), so a ban may affect more than one person.

## Published Docker images

GitHub Actions workflow `.github/workflows/docker-publish.yml` pushes multi-platform amd64/arm64 images on branch pushes and version tags. The image tag for the feature branch is:

`ghcr.io/nekosuneprojectsforks/cliamp-server:feature-nekosune-radio-dashboard`

After merge to main, pull:

`ghcr.io/nekosuneprojectsforks/cliamp-server:latest`

Package visibility is managed separately in GitHub Packages. A successful Actions run does not guarantee anonymous pulls until the package is public.

## Public API documentation

The public web reference is served at `/docs`, with sample JSON, JavaScript, curl and a summary of privacy boundaries. It links to the detailed repository API document. The public endpoints do not provide individual listener IP addresses.

## Configuring country charts

Add both entries to your mounted `config.toml`:

```toml
[stats]
db_path = "/data/stats.db"
min_session = 10

[geo]
db_path = "/data/GeoLite2-City.mmdb"
```

Mount `./data:/data` read-write. Download a compatible **City** GeoIP MMDB file and place it at `./data/GeoLite2-City.mmdb` (the exact filename must match). The non-root container must be able to write to `/data` for SQLite. Restart the server after changing settings. Check `/statistics`: it should return JSON, not 404. Listening session country statistics are persisted on disconnect after the minimum session duration; they cannot be backfilled from unrecorded prior connections. A missing or invalid GeoIP file can prevent startup.

## Configurable HTTP GeoIP providers

Set the HTTPS API endpoint and map the JSON response fields to the standard geo information used for statistics. Nested JSON fields can be mapped using dotted paths (for example `location.country.name`).

```toml
[geo]
api_url = "https://api.nekosunevr.co.uk/v5/proxy/iplookup"
api_param = "ip"
api_ip_field = "ip"
api_country_field = "country"
api_country_code_field = "countryTag"
api_city_field = "city"
api_latitude_field = "latitude"
api_longitude_field = "longitude"
api_valid_field = "valid"
api_private_field = "is_private"
db_path = "/data/GeoLite2-City.mmdb"
```

`api_url` can instead point to another provider that returns a JSON object; change the mappings for that provider. `api_param` is the GET query parameter that receives the listener IP. Unset mappings default to NekoSune names (country, countryTag, city, latitude, longitude). Set optional `api_ip_field`, `api_valid_field`, and `api_private_field` to enforce response checks when supported.

Requests use a two-second timeout, cache successful lookups for 24 hours in memory, and fall back to the local MMDB if the API fails. Keep `db_path` blank to use API-only mode. `[stats] db_path` must still be set to accumulate historical listener locations. Country chart updates when recorded sessions disconnect. External IP lookup providers receive listeners' IP addresses, so use only a provider you trust.
