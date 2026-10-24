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
- New music uploads and music deletions refresh the upcoming playlist without requiring a restart.
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

## Modern admin workspace and live media management

The `/admin` area now has a sidebar and separate Overview, Radio Stations, Music Library, Ads & Jingles, Live Listeners, and IP Moderation views. Select a station to upload, review, or permanently delete audio files. The music library uses a mutex-protected in-memory playlist replacement: currently streaming listeners remain connected and the encoder is not intentionally restarted. An already-prefetched next track may play once before the replacement takes effect.

To enable separate advertisement and jingle libraries for each station, create writable directories, mount them into Docker, and configure each station:

```toml
[stations.neffex]
name = "Neffex Station"
path = "/music/neffex"
shuffle = true
ads_path = "/media/neffex/ads"
ad_every_n_songs = 5
ad_shuffle = true
jingles_path = "/media/neffex/jingles"
jingle_every_n_songs = 2
jingle_shuffle = true
```

Mount `./media:/media` with permissions allowing the non-root container process to write. Each media path must exist before startup, but may initially be empty. The admin dashboard can upload and delete multiple audio assets from each directory; a scheduler pool reload updates the next media choice without restarting the encoder.

**Limitations:** Creating and removing entire stations is not yet implemented through the admin UI. Stations are still defined in `config.toml` and need a restart for changes to take effect. The existing station Stop control blocks new listener connections but does not halt the running encoder. There is currently no independent per-asset ad weighting, ad campaign targeting, or per-song jingle insertion rule; jingles are periodic, based on the count of regular music tracks. Uploaded files must be media you are authorised to stream.

## Statistics persistence across container restarts

Historical listening sessions, country summaries, and all-time peak listener counts are stored in SQLite at `[stats] db_path = "/data/stats.db"`. Mount `./data:/data` (or a persistent named Docker volume), keep the volume between container replacements, and grant container UID/GID 65532 permission to write. Do not mount `/data` as ephemeral tmpfs or delete the SQLite database or its accompanying `-wal` and `-shm` files while the server is running. When restoring backups, use a consistent SQLite backup rather than copying only the live database file in WAL mode.

The public globe now labels **historical sessions** and **all-time peak listeners** separately from **currently connected listeners**. The former persist in SQLite; live listener connections naturally disconnect and return to zero on reboot until new listeners connect. Country dots represent active listeners only and do not imply that historical visitors remain online.

## Low-latency live radio streaming

The MP3 ring now uses approximately 1.5 seconds of pre-roll for joining listeners rather than approximately 3.3 seconds. Stream handlers send `X-Accel-Buffering: no` and `Cache-Control: no-cache, no-store, no-transform`, and resynchronise connections if their server-side stream position trails live by more than roughly 7 seconds. The audio encoder remains continuous; this does not guarantee a fixed listener latency.

**Reverse proxy:** Nginx must not buffer streams. For stream routes, set `proxy_buffering off;`, `proxy_request_buffering off;`, `gzip off;` and `proxy_read_timeout 3600s;`. On Cloudflare or other CDNs, bypass caching/transforming audio stream responses. Player-side buffering can still cause tens of seconds of latency; VLC can be tested with an adjusted network caching setting (e.g. 1000–2000 ms) without changing the server. The standard HTML5 audio element does not provide an exact portable buffer-size control, so ~3–5-second end-to-end delivery is a target, not a guaranteed SLA.

## Remote MinIO / S3 station libraries (draft integration)

S3-backed stations list objects using the S3-compatible API and send **short-lived signed HTTPS GET URLs directly to FFmpeg**. The audio is consumed as a network stream into the station encoder; there is no local music-file mirror, FUSE mount, or Docker bucket sync. The encoder necessarily buffers audio in RAM during playback. The S3 bucket remains authoritative.

Use a station-specific nested TOML configuration (do not put secret values in TOML):

```toml
[stations.s3music]
name = "Remote Music"
genre = "Radio"
shuffle = true
[stations.s3music.s3]
endpoint = "https://minio.example.com"
bucket = "radio-music"
prefix = "music/"
region = "us-east-1"
access_key_env = "MINIO_ACCESS_KEY"
secret_key_env = "MINIO_SECRET_KEY"
```

Set the actual `MINIO_ACCESS_KEY` and `MINIO_SECRET_KEY` environment variables in your Docker deployment or secure secret management; never post their values publicly. Provide a dedicated MinIO access key scoped to the necessary bucket/prefix, allowing ListBucket and GetObject, plus PutObject/DeleteObject if you want web-managed uploads/deletions.

The radio must be able to contact its MinIO endpoint over HTTPS. Do not use a MinIO console URL; use its **S3 API endpoint**. A MinIO/S3 backend is optional: local library stations still work.

The admin's create/delete station form persists changes to `CLIAMP_STATIONS_FILE` (default `/data/stations.json`). This must be on a persistent writable volume. **Creating and removing stations requires an explicit process restart** to register/unregister routes and start/stop station encoders; there is no runtime hot-add yet. Deleting the station entry does **not** delete any music files or objects. For S3-backed stations, adding/deleting music via Music Library operates on remote S3 and refreshes upcoming rotation.

Restrictions: The remote S3 music catalog currently uses object filenames for title information rather than parsing embedded ID3 tags over the network; private objects are never served directly to public users. S3 outages and expired credentials can interrupt future track reads, so ensure network reliability. Presigned URL lifetime is 3 hours; audio longer than that may need a longer or renewable access strategy. Test on a private development station before merging.
