# Radio dashboard: sources, persistence and video links

## What changed

- Removed the unsolicited **Worldwide Radio Directory** section and its public
  Radio Browser discovery routes. The homepage now lists only your stations.
- Fixed station creation: the browser sends the selected `source` field.
- The station creation form hides irrelevant inputs. A local station path is
  optional and auto-generates `/music/<station-id>`; the server creates that
  folder if the runtime user can write to the parent music volume.
- Empty local or YouTube stations start broadcasting quiet standby audio instead
  of crashing on startup. Add music from **Music library** to replace standby.
- New stations show up immediately as **Pending restart** in the admin list.
  They do not appear on the public homepage until the Go process restarts.
- **YouTube videos (yt-dlp)** is an optional source. In **Music library** choose
  a YouTube station and use **Add YouTube video** to enter one HTTPS video URL.
  Add/remove operations persist the playlist and refresh the upcoming rotation
  without restarting an already-active station.
- Existing S3/MinIO, remote HTTPS URLs and local media remain available.

## First-time Docker setup

Use `compose.yaml` for **persistent volumes**, and provide a config file
at `./config.toml`. Example minimal content:

```toml
[server]
host = "0.0.0.0"
port = 8000

[admin]
password = "REPLACE_WITH_A_LONG_UNIQUE_PASSWORD"

[stations.radio]
name = "NekoSune Radio"
path = "/music/radio"
shuffle = true
```

The default named volumes are `cliamp_server_music` and
`cliamp_server_persistent_data`. The image creates a writable `/data`
directory, and when `/data` exists the default historical SQLite database is
`/data/stats.db`. Managed station definitions are at `/data/stations.json`;
bans at `/data/banned-ips.json`.

To start with the above configuration, create `/music/radio` in the image
or mounted music volume before the first startup. You can also use an existing
local station configured with a writable path. Later stations added through
the UI create their folders automatically.

Do **not** delete the named data volume when upgrading or recreating containers.
When switching from an old deployment, back up and migrate the existing
`stats.db`, `stations.json`, and `banned-ips.json` into the new volume.
Do not copy only the SQLite DB while it is actively being written; use the
SQLite backup API or stop the server cleanly first.

### What is preserved vs reset?

| State | Restart behaviour |
| --- | --- |
| Historical sessions | Persistent in SQLite if the volume is retained |
| All-time peak | Persistent in SQLite if the volume is retained |
| Listener connections | Reset: users must reconnect |
| Current-country dots | Reset: represent live listeners, not past visitors |
| Station definitions | Persistent in stations.json if the volume is retained |
| YouTube URL playlists | Persistent in stations.json |
| Uploaded music | Persistent in /music or S3 |
| IP bans | Persistent in /data/banned-ips.json |

## YouTube prerequisites

The Docker image includes FFmpeg, yt-dlp, its Python dependencies and Node.js
for JavaScript challenges. A video URL must be an ordinary
`https://www.youtube.com/watch?v=<11-character-id>` or
`https://youtu.be/<11-character-id>` link. A YouTube **playlist URL** is
not currently imported as a whole.

yt-dlp resolves a fresh playable audio URL when the station reaches each
video. Availability may change due to YouTube restrictions, geographic
licensing, authentication, or challenge handling. The source is not guaranteed
to work for every video and there is no account-cookie import in this release.

Use this only with media you are allowed to rebroadcast. Downloading or
resolving a video does not grant permission to create a public radio broadcast.
Spotify, YouTube Music, Tidal, Qobuz, Mixcloud and other subscription services
are not automatically available as unrestricted radio sources.

## Public directory vs publishing a station to directories

The homepage **does not discover third-party stations**. This is separate from
the option to publish your own stream through an Icecast2 server. See
[multisource-radio.md](multisource-radio.md) for Icecast publisher settings.
SHOUTcast2 publishing and public listing status are not yet implemented.

## Verification

Run `go test ./...`, `go build ./...`, and Docker build and smoke tests.
Confirm a new empty local station is created, it becomes active after restart,
and adding a track replaces standby audio. On a YouTube station, enter a
rights-cleared video URL and verify actual playback rather than just successful
API storage. Restart and confirm the URL and historical listener totals persist.
