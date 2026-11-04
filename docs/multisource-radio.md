# Multi-source radio and Icecast publishing

The existing server supports three input modes per station: a local directory
(`path`), an S3-compatible object store (`[stations.<id>.s3]`) or a list of
direct HTTPS audio files (`remote_urls`). These are **alternatives**, not yet
a combined playlist. S3 support was present before this change.

The new `[[stations.<id>.relays]]` destination publishes the same continuous
128-kbps MP3 station output to an external **Icecast2** server. There can be
multiple independent Icecast destinations per station. Failure of a remote
destination does not stop local playback. Each relay retries every 5 seconds.

## Example (S3 input, two Icecast destinations)

```toml
[server]
host = "0.0.0.0"
port = 8000

[stations.radio]
name = "NekoSune Radio"
description = "Community music"
genre = "Electronic"
shuffle = true

[stations.radio.s3]
endpoint = "https://s3.example.com"
region = "us-east-1"
bucket = "music"
prefix = "radio/"
access_key_env = "S3_ACCESS_KEY"
secret_key_env = "S3_SECRET_KEY"

[[stations.radio.relays]]
type = "icecast2"
name = "NekoSune Radio"
host = "icecast.example.com"
port = 8000
mount = "/live.mp3"
password_env = "ICECAST_SOURCE_PASSWORD"
tls = false
public = true

[[stations.radio.relays]]
type = "icecast2"
name = "NekoSune Radio Backup"
host = "backup.example.com"
port = 8443
mount = "/radio.mp3"
password_env = "ICECAST_BACKUP_PASSWORD"
tls = true
public = false
```

Set `S3_ACCESS_KEY`, `S3_SECRET_KEY`, `ICECAST_SOURCE_PASSWORD`, and
`ICECAST_BACKUP_PASSWORD` in the **container/server environment**, not in Git.
Only use `tls = true` if the destination supports Icecast-over-TLS. TLS is
recommended because source passwords otherwise travel without encryption.

For direct HTTPS media instead of S3, replace the S3 block with:

```toml
remote_urls = [
  "https://media.example.com/licensed-track-1.mp3",
  "https://media.example.com/licensed-track-2.flac",
]
```

For local files, use `path = "/music"` as documented in the README.

## Public radio directories

`public = true` sets the **Icecast source public flag**. It does NOT guarantee
listing. The remote Icecast2 server must enable its YP directory configuration,
and directory operators decide whether and when to index a station. The
station also needs a public reachable URL, stable uptime, accurate metadata
and relevant permissions/licences. SHOUTcast DNAS2 source authentication and
its directory registration are **not yet implemented** by this change.

## Planned sources (not included in this PR)

- Explicit source selection and mixed playlists (local, S3, remote HTTPS).
- yt-dlp URL inspection for personal or rights-cleared media (requires yt-dlp
  in the container, URL controls, auth/cookie handling and upstream policy review).
- Authorized Plex, Jellyfin, Emby, Navidrome/Subsonic, Audiobookshelf, etc.
- Services such as Spotify, Tidal, Qobuz and YouTube Music have distinct
  API/playback/DRM terms; catalog access does not imply public rebroadcast rights.
- Podcast/RSS, PLS/M3U import and internet-radio relay inputs.
- Separate SHOUTcast 2 output, directory configuration and metadata updates.
- Cover-art search via the no-key iTunes Search API, licensed cover images,
  and operator uploads; do not assume search results are free to redistribute.

For public radio, feed the station only music you have rights to broadcast.
An extractor working technically is not evidence of retransmission permission.
