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

`GET /api/public/radio` returns all stations with active listener counts, stream URLs, and current track metadata. Each `current_track` includes `cover_url`: an HTTPS iTunes artwork URL if a confident song match is found, or an empty string when unavailable. No listener IPs are included. Album covers are cached in memory (24 hours for a match, 6 hours for a miss). The server's connection to iTunes has a 3-second timeout.

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
        "cover_url": "https://is1-ssl.mzstatic.com/image/thumb/example/600x600bb.jpg"
      },
      "stream": "/neffex/stream"
    }
  ],
  "total_listeners": 2
}
```

Artwork URL shown above is illustrative; clients should use the value returned by the API and handle an empty URL.


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
