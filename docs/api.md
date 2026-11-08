# API

## Now-playing metadata in VLC, M3U, PLS and Python

Open `/streams.m3u` or `/streams.pls` in VLC, then select a station. Each playlist contains stream URLs; the playlist text is a static station directory. The live MP3 stream `/{id}/stream` supplies the changing now-playing title through **ICY metadata**, not through updates to the playlist file. A compatible client requests `Icy-MetaData: 1`, reads `icy-metaint`, and parses 16-byte-multiple metadata blocks. The title format is `StreamTitle='Artist - Title';`. VLC's display behaviour can vary by version and settings.

To confirm the response includes the ICY metadata interval:

```bash
curl -s -D - -o /dev/null --max-time 3 -H 'Icy-MetaData: 1' https://radio.nekosunevr.co.uk/neffex/stream
```

Because streams have no end, a timeout is expected. A Python listener can use the following example to read a single metadata block:

```python
import urllib.request

url = "https://radio.nekosunevr.co.uk/neffex/stream"
req = urllib.request.Request(url, headers={"Icy-MetaData": "1", "User-Agent": "NekoSuneAI/1.0"})
with urllib.request.urlopen(req, timeout=15) as stream:
    interval = int(stream.headers.get("icy-metaint", "0"))
    if not interval:
        raise RuntimeError("ICY metadata unavailable")
    def read_exact(n):
        data = bytearray()
        while len(data) < n:
            part = stream.read(n - len(data))
            if not part:
                raise EOFError("stream ended")
            data.extend(part)
        return bytes(data)
    read_exact(interval)
    size = read_exact(1)[0] * 16
    print(read_exact(size).rstrip(b"\x00").decode("utf-8", "replace") if size else "No metadata in this block")
```

For browsers, apps, and bots that need a direct JSON response rather than reading audio, use `/api/public/radio` and `current_track.title`, `current_track.artist`, and `current_track.cover_url`.


## NekoSune public radio API

`GET /api/public/radio` returns all stations with active listener counts, stream URLs, and current track metadata. Every `current_track.cover_url` points to the same local `/api/{station}/art.png` resource used by the single-station API and HLS visuals. No listener IPs are included. Matching external artwork is resolved by a background per-station worker and saved as a validated PNG on the persistent `/data` volume; public HTTP requests do not query iTunes.

```json
{
  "stations": [
    {
      "id": "neffex",
      "name": "Neffex Station",
      "description": "",
      "genre": "Music",
      "listeners": 2,
      "tracks": 40,
      "online": true,
      "current_track": {
        "title": "Example Song",
        "artist": "Example Artist",
        "album": "Example Album",
        "cover_url": "https://radio.nekosunevr.co.uk/api/neffex/art.png"
      },
      "stream": "/neffex/stream"
    }
  ],
  "total_listeners": 2
}
```

The artwork URL is an illustrative local endpoint; the exact origin is derived from the incoming request. When there is no matching cover, the generated CD-in-a-jewel-case fallback is returned as a PNG.


## Single-station public radio endpoints

For bots, overlays, dashboards and restream services that need **one radio station** instead of the entire `/api/public/radio` directory:

| Endpoint | Format | Purpose |
|---|---|---|
| `GET /api/neffex` | JSON | One station's public details and current song |
| `GET /api/neffex/` | JSON | Same as above |
| `GET /api/neffex/nowplaying` | JSON | One station's current details and current song (same shape, no full list) |
| `GET /api/neffex/art.png` | Image | Current cover, or generated CD jewel-case PNG when unavailable |

Replace `neffex` with any configured station ID. The API uses the live station registry and does not hard-code the names. Unknown stations return **404**; endpoints are read-only, require no API key and return `Cache-Control: no-store`. Web apps can read them cross-origin (CORS).

The response for `/api/neffex/nowplaying` is a **single station object** with the same field names as items in the existing `/api/public/radio` list:

```json
{
  "id": "neffex",
  "name": "Neffex Station",
  "description": "Neffex 24/7",
  "genre": "neffex",
  "listeners": 2,
  "tracks": 40,
  "online": true,
  "current_track": {
    "title": "Get Through",
    "artist": "Neffex",
    "album": "Built to Last: The Collection",
    "cover_url": "https://radio.nekosunevr.co.uk/api/neffex/art.png"
  },
  "stream": "/neffex/stream"
}
```

This example is illustrative, **not** a live result. Listener counts, total tracks and songs come from the live station. All radio endpoints return the same **local** cover URL. The artwork background worker searches for a matching cover only when the song changes or an uncached match is retried, downloads it from an approved iTunes artwork host, and saves a normalised per-song PNG. `/api/neffex/art.png`, `/<station>/art.png`, and HLS video then reuse that local cache **without fetching the remote image during HTTP requests or every HLS frame**. The selection order is per-song art, operator-configured station cover, and finally the generated CD jewel-case fallback. Song changes update the cached `art.png` atomically without restarting the radio encoder. The fallback is created once per station on startup and remains available if network lookup fails.

The existing `/api/public/radio`, `/api/v1/radios`, audio streams and admin APIs remain unchanged.

## Stream Endpoints

Each station exposes three endpoints under its ID prefix:

| Endpoint | Content Type | Description |
|----------|-------------|-------------|
| `/<id>/stream` | `audio/mpeg` | Live MP3 audio stream with ICY metadata |
| `/<id>/stream.pls` | `audio/x-scpls` | PLS playlist file pointing to the stream |
| `/<id>/stream.m3u` | `audio/x-mpegurl` | M3U playlist file pointing to the stream |
| `/streams.pls` | `audio/x-scpls` | PLS playlist file listing all stations |
| `/streams.m3u` | `audio/x-mpegurl` | M3U playlist file listing all stations |
| `/stations` | `application/json` | Station directory with stream URLs and exposed track counts |
| `/logo.svg` | `image/svg+xml` | CLIAMP station logo for players and directory listings |

`/stations` lists every station in config file order. `tracks` is the number of tracks the station exposes, and `tracks_url` is present only when the station has `expose_tracks = true`. A client can use this document to decide which stations it opens as a playlist and which it plays as a live stream.

```json
{
  "stations": [
    {
      "id": "lofi",
      "name": "Lofi",
      "description": "Lofi 24/7",
      "genre": "Lofi",
      "stream": "https://radio.example/lofi/stream",
      "tracks": 0
    },
    {
      "id": "omarchy",
      "name": "Omarchy",
      "genre": "Electronic",
      "stream": "https://radio.example/omarchy/stream",
      "tracks": 33,
      "tracks_url": "https://radio.example/omarchy/tracks"
    }
  ]
}
```

The stream endpoint supports ICY metadata. Clients that send the `Icy-MetaData: 1` request header receive inline metadata blocks containing the current track title and artist.

Response headers include `icy-name`, `icy-genre`, `icy-br` (bitrate), `icy-sr` (sample rate), and `icy-metaint` (metadata interval). The `icy-logo` extension contains an absolute URL to `/logo.svg`; use the same URL as the station favicon when registering it with a radio directory.

## Track Endpoints

Available for stations configured with `expose_tracks = true`.

| Endpoint | Description |
|----------|-------------|
| `/<id>/tracks` | JSON library listing with a persistent `plays` count for each track |
| `/<id>/tracks.m3u` | M3U playlist containing every track |
| `/<id>/tracks/<track-id>` | Direct audio file with range request support |
| `/<id>/tracks/statistics` | Aggregated play statistics for the exposed tracks of one station |
| `/tracks/statistics` | Aggregated play statistics for the exposed tracks of all stations |

Play counts and track statistics require `--stats-db`. A play is counted when a `GET` request starts at byte 0. `HEAD` requests and seeks into the middle of a track are not counted. A new request for the same track from the same client IP address within 30 seconds is not counted, because players often send two requests to start one track. Without a statistics database, every track reports `"plays": 0` and the statistics routes return 404.

### Track Statistics

The track statistics endpoints are public and contain no IP addresses. They use the same structure as the radio statistics, with plays in place of sessions.

```
curl http://localhost:8000/radio/tracks/statistics
curl http://localhost:8000/tracks/statistics
```

The server sees file requests, not playback. For this reason, the fields have these meanings:

- `total_plays` counts every recorded play, including plays from before the upgrade that added track statistics.
- `active_listeners` counts the client IP addresses that started a track on the station in the last 10 minutes.
- `peak_listeners` is the all-time highest `active_listeners` value. Track peaks are separate from radio peaks.
- `top_countries`, `top_cities` and `daily` count plays. They include only plays recorded after the upgrade. Country and city fields require GeoIP.
- `top_tracks` lists up to 10 tracks with the most plays, in the same format as `/<id>/tracks`.
- Track statistics have no listen hours, because the server cannot measure playback time for a downloaded file.

Per-station response:

```json
{
  "total_plays": 1520,
  "peak_listeners": 14,
  "active_listeners": 3,
  "active_listener_countries": [
    { "country": "Norway", "country_code": "NO", "listeners": 2 }
  ],
  "top_countries": [
    { "country": "Norway", "country_code": "NO", "plays": 410 }
  ],
  "top_cities": [
    { "city": "Oslo", "country_code": "NO", "plays": 120 }
  ],
  "top_tracks": [
    {
      "id": "9a5501dc8bfe5544",
      "title": "Song Title",
      "artist": "Artist Name",
      "filename": "song.mp3",
      "url": "https://radio.example/radio/tracks/9a5501dc8bfe5544",
      "plays": 87
    }
  ],
  "daily": [
    { "date": "2026-09-28", "plays": 64 }
  ]
}
```

Global response:

```json
{
  "total_plays": 2210,
  "peak_listeners": 19,
  "stations": {
    "radio": {
      "total_plays": 1520,
      "peak_listeners": 14,
      "active_listeners": 3,
      "active_listener_countries": [],
      "top_countries": [],
      "top_cities": [],
      "top_tracks": [],
      "daily": []
    }
  }
}
```

## Status Endpoints

| Endpoint | Description |
|----------|-------------|
| `/<id>/status` | JSON status for a single station |
| `/status` | JSON status for all stations |

Status endpoints require an admin Bearer token for remote access. Direct requests arriving from the loopback interface with a loopback Host (`127.0.0.1`, `::1`, or `localhost`) and no proxy forwarding headers may read `/status` and `/<id>/status` without a password. If `[admin] password` is unset, remote requests return HTTP 503. Public consumers should use `/api/public/radio`:

```
curl -H "Authorization: Bearer yourpassword" http://localhost:8000/status
```

### Per-Station Status Response

```json
{
  "station": "Pop Station",
  "favicon": "https://radio.example/logo.svg",
  "listeners": 12,
  "listener_details": [],
  "current_track": {
    "title": "Song Title",
    "artist": "Artist Name",
    "album": "Album Name"
  },
  "uptime": "5h30m45s",
  "uptime_seconds": 19845,
  "playlist_length": 247
}
```

### Global Status Response

```json
{
  "stations": {
    "pop": {
      "name": "Pop Station",
      "favicon": "https://radio.example/logo.svg",
      "listeners": 12,
      "listener_details": [],
      "current_track": { "title": "", "artist": "", "album": "" },
      "playlist_length": 247
    },
    "jazz": {
      "name": "Jazz Station",
      "favicon": "https://radio.example/logo.svg",
      "listeners": 3,
      "listener_details": [],
      "current_track": { "title": "", "artist": "", "album": "" },
      "playlist_length": 89
    }
  },
  "total_listeners": 15,
  "uptime": "5h30m45s",
  "uptime_seconds": 19845
}
```

## Statistics Endpoints

Available when `--stats-db` is configured. These are **public** (no password required) and contain no IP addresses.

| Endpoint | Description |
|----------|-------------|
| `/<id>/statistics` | Aggregated listener statistics for a single station, including its all-time listener high |
| `/statistics` | Aggregated listener statistics for all stations, including the global all-time listener high |

```
curl http://localhost:8000/radio/statistics
curl http://localhost:8000/statistics
```

### Per-Station Statistics Response

```json
{
  "total_sessions": 8234,
  "total_listen_hours": 2810.3,
  "peak_listeners": 68,
  "active_listeners": 42,
  "top_countries": [
    { "country": "Norway", "country_code": "NO", "sessions": 3200, "listen_hours": 1100.2 }
  ],
  "top_cities": [
    { "city": "Oslo", "country_code": "NO", "sessions": 800, "listen_hours": 280.5 }
  ],
  "daily": [
    { "date": "2026-03-05", "sessions": 150, "listen_hours": 48.2 }
  ]
}
```

### Global Statistics Response

```json
{
  "total_sessions": 12847,
  "total_listen_hours": 4231.5,
  "peak_listeners": 86,
  "stations": {
    "pop": {
      "total_sessions": 8234,
      "total_listen_hours": 2810.3,
      "peak_listeners": 68,
      "active_listeners": 42,
      "top_countries": [],
      "top_cities": [],
      "daily": []
    }
  }
}
```


## Shared, local-only artwork cache for API and HLS

All public station endpoints expose `/api/{station}/art.png` as the cover URL:

- `GET /api/public/radio` → `stations[].current_track.cover_url`
- `GET /api/{station}/nowplaying` → `current_track.cover_url`
- `GET /api/v1/radios` → `radios[].artwork_url`
- `GET /api/v1/radios/{station}` → `artwork_url`

The original `/{station}/art.png` and VRChat HLS visual frames use exactly the same on-disk `art.png`. A background service updates the per-song cache only for uncached tracks, throttles misses, and refreshes each station's current image every five seconds, including stations with HLS disabled. No web requests originate from the HLS frame generator or public API handlers. All artwork is normalized to PNG when saved. The worker stops when the HTTP server shuts down.

The per-station files under `/data/vrchat-assets/{station}/` persist across container restarts when the Docker data volume is mounted. Manual per-track artwork and custom station covers take priority over automatically fetched artwork and the CD fallback. When there is no music-specific image, users see a reflective compact disc inside a jewel case rather than a blank image.

## Persistent artwork match database (all sources)

Album-cover matches are indexed in **one SQLite artwork database** at `/data/artwork-cache.sqlite` (override: `CLIAMP_ARTWORK_CACHE_DB`). Each record holds the **station ID, source type, canonical source identity, artist, title, album, matching external cover URL, checked time and result status**. Source types are `youtube`, `local`, `s3`, `remote` and `metadata` (for a source that has no file identity yet).

YouTube watch/short URLs with the same 11-character video ID share a source identity. Local files and S3 objects use their own source paths. Two tracks with identical names but different audio sources retain **separate database rows and separate source-specific PNG files**, preventing accidental artwork swaps. Changing a track's artist/title also changes its match key; album metadata updates do not lose an existing cached match.

- When the track is already in the database as `found`, the server reads its saved source-specific PNG without contacting the iTunes search API.
- When the row is `missing`, the server uses the CD jewel-case fallback without repeating catalogue searches on each playback or restart.
- If a PNG has disappeared but the database still contains an approved artwork URL, the background worker can **restore the PNG from that URL without a new search**.
- A new uncached track is looked up once in the background, then the match/miss is persisted.
- Cover images remain PNG files in the station's existing persistent `/data/vrchat-assets/{station}/` location; the SQLite file holds source identities and checked URL results, not large image blobs. Manually uploaded track covers always take priority over automatically fetched covers.
- Public HTTP handlers, the web player, and HLS video never query iTunes directly.

### Admin-only recheck of album artwork

The Admin dashboard has a **Refresh album covers** button for every station. It starts a single background job for the whole station library, handles **YouTube, local files, S3 and remote tracks**, and processes images **one by one**. The operator can follow completed/total counts and see found, missing and failed items. The job does not restart audio or the HLS encoder.

| Endpoint | Purpose |
|---|---|
| `GET /api/admin/stations/{station}/artwork/cache` | View matched/missing/failed database counts and refresh progress |
| `POST /api/admin/stations/{station}/artwork/refresh` | Explicitly rescan all current library sources and refresh artwork matches |

Both endpoints require the same **admin Bearer token** as the other management endpoints. Refreshing bypasses existing catalogue matches and may replace automatically cached artwork while **preserving manually uploaded covers**. Only one rescan job may run at a time across all stations to protect Raspberry Pi CPU and network capacity; a concurrent request returns HTTP 409.

Example:
```bash
curl -H "Authorization: Bearer YOUR_ADMIN_PASSWORD" \
  https://radio.nekosunevr.co.uk/api/admin/stations/neffex/artwork/cache
```

Cache storage is persistent when the server's `/data` volume is mounted. Keep `artwork-cache.sqlite`, `youtube-source-cache.sqlite` (YouTube metadata history) and `vrchat-assets` together in backups.


## Optional on-demand music library and playlist mode

The 24/7 radio is no longer the only listening option. Each station can optionally offer **individual songs** in a searchable browser music player, a JSON track list and an extended M3U playlist. This is built into CLIAMP Server (not a new music host). It is distinct from `/{station}/stream` and does not interrupt the live station.

### Enable per station

Song-level streaming is deliberately **off by default**, because allowing listeners to play individual files can expose media that the station owner intended only for live radio. Under `/admin`, select **Enable on-demand playlist** for the station, then restart CLIAMP Server to activate the HTTP routes. You can also enable it in the station's TOML config with `expose_tracks = true`. For managed stations, the admin page persists this choice for you. Disabling it also takes effect after restart.

Once enabled, the public URLs are:

| URL | Result |
|---|---|
| `/library/neffex` | Browser music player: search, play song, previous, next and shuffle, with an optional Play live radio button |
| `/api/neffex/playlist` | JSON library including source type, song metadata, stable playback URLs and availability |
| `/api/neffex/playlist.m3u` | Extended M3U with station-hosted MP3 track URLs |
| `/api/neffex/playlist.pls` | Shoutcast/Winamp-style PLS containing the same stable MP3 URLs |
| `/neffex/tracks` | Existing per-station JSON track index |
| `/neffex/tracks.m3u` | Existing extended M3U library |
| `/neffex/tracks/{opaque-id}` | Individual local audio, with HTTP range/seek support for directly supported file formats |
| `/neffex/tracks/{opaque-id}.mp3` | Individual MP3 playback; automatically transcodes non-MP3 audio and supported YouTube sources |

The library index tracks the station's current playback playlist and updates when tracks are added or removed through management operations, without restarting live audio. Song IDs are opaque hashes of the source reference; filesystem locations and signed upstream URLs never appear in the public response. The player does not force you to listen live.

### YouTube and car-radio compatibility

YouTube videos are **not downloaded into permanent media files**. When a listener opens one of the stable `/neffex/tracks/{opaque-id}.mp3` links, CLIAMP invokes the existing `yt-dlp --output -` and FFmpeg pipeline, sending decoded/re-encoded audio over the radio server's HTTP connection. This avoids publishing short-lived `googlevideo` URLs in playlists, even when YouTube rotates its signed media link. Audio retrieval takes place on the CLIAMP server using its own outbound network connection and IP, **not the listener's IP**. No technique can guarantee that YouTube will allow extraction from every ISP/VPS address; removed/private/geoblocked/DRM-protected content, provider throttling or yt-dlp breakage can still make a song unavailable. Do not use this feature to bypass access controls. Also observe music licensing and provider terms.

For compatibility with low-resource hardware, on-demand FFmpeg conversion is limited to **four concurrent connections**, with **two YouTube sources maximum**. Additional requests return HTTP 503 rather than saturating a Raspberry Pi. Local MP3 files are served directly with `Accept-Ranges: bytes` and can be seeked; converted or YouTube audio is an indefinite MP3 response without byte-range seeking or an ahead-of-time Content-Length. **Some car radios require length/range support**, so compatibility is player-dependent. For the widest car-radio compatibility, store licensed music locally as MP3 and enable the library playlist. HLS/VRChat live visuals continue operating separately.

There is no remote YouTube URL in the public track list: the returned URLs remain on `radio.nekosunevr.co.uk`. Stable addresses do not guarantee that upstream YouTube audio is always retrievable.

