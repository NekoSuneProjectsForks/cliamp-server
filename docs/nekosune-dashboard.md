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
