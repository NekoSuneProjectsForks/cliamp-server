# cliamp-server

An internet radio streaming server written in Go. Shoutcast/ICY compatible, multi-station, live metadata, on-the-fly transcoding, ad injection, GeoIP tracking, and listener statistics.

Point it at a directory of audio files and it starts broadcasting.

Looking for the client? See [cliamp](https://github.com/bjarneo/cliamp).

## Quick Start

Requires `ffmpeg` with `libmp3lame` on `PATH`.

```
go build
./cliamp-server --music /path/to/your/mp3s
```

Open `http://localhost:8000/radio/stream` in VLC or any Shoutcast-compatible player.

## Multi-Station

Create `~/.config/cliamp-server/config.toml`:

```toml
[server]
port = 8000

[stations.pop]
name = "Pop Station"
path = "/music/pop"
shuffle = true

[stations.jazz]
name = "Jazz Station"
path = "/music/jazz"
shuffle = true
```

```
./cliamp-server
```

Each station gets its own stream at `/<id>/stream`.

## Docker

```
docker build -t cliamp-server .
docker run -d -v /path/to/music:/music:ro -p 8000:8000 cliamp-server --music /music
```

## Documentation

- [Configuration](docs/configuration.md) — CLI flags, TOML config, station options
- [API](docs/api.md) — Stream endpoints, status endpoints, response format
- [Features](docs/features.md) — Audio format support, ad scheduling, GeoIP, listener statistics
- [Deployment](docs/docker.md) — Docker, nginx reverse proxy
- [Multi-source radio & Icecast publishing](docs/multisource-radio.md) — S3/HTTPS inputs, external relay configuration, and planned providers

## License

See LICENSE file.


### Optional on-demand music playlists

Every radio station can optionally expose **its individual music tracks**, so listeners don't have to join the live broadcast. This is **disabled by default** to avoid publishing individual media files unintentionally.

In the Admin dashboard choose **Enable on-demand playlist** for a station and restart the container, or set `expose_tracks = true` in that station's TOML configuration. You will then have a searchable, NekoSune-themed web music player at `/library/<station>`, a JSON music library at `/api/<station>/playlist`, and downloadable M3U/PLS playlists at `/api/<station>/playlist.m3u` and `/api/<station>/playlist.pls`. The website will link to the player for enabled stations.

Individual songs are served through stable station-hosted URLs. Local MP3s support HTTP byte-range seeking. YouTube, S3, remote audio, and non-MP3 local formats stream through the existing FFmpeg conversion path to `/STATION/tracks/TRACK_ID.mp3`. YouTube extraction runs on demand with `yt-dlp` piped to FFmpeg; media **is not stored permanently**, signed provider links are never returned to listeners, and video availability still depends on the source and the server's outbound IP. The server limits concurrent on-demand conversion to four clients, including no more than two YouTube conversions. Some car radios require a known content length or seek support; for maximum compatibility use licensed local MP3 files rather than relying exclusively on YouTube.

Each song has its own local cached artwork URL at `/api/<station>/playlist/TRACK_ID/art.png`; the image matches the source-specific artwork database and falls back to the CD jewel case where necessary. Live radio and HLS remain independent. See [API documentation](docs/api.md) for endpoint details.
