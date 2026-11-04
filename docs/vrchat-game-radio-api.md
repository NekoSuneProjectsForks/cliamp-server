# VRChat and game radio integration

## Why the raw MP3 URL may buffer in VRChat

`/<station>/stream` is a real-time, indefinite MPEG Layer III (MP3)
HTTP stream with ICY metadata support. It is intended for radio players.
VRChat's **AVPro video player** is a better choice for live streams than the
Unity video player. MP3-only live streaming has platform-dependent player
compatibility, and reverse proxies may affect streaming as well. Do not
interpret a browser/VLC success as proof that VRChat accepts the stream.

An optional `vrchat_hls = true` station setting adds an **H.264 + AAC HLS**
endpoint suitable for testing with VRChat AVPro. It generates tiny black video
at 320x180/1 FPS plus audio, in MPEG-TS segments. MP3 broadcasting and existing
listeners are unchanged. Each enabled station creates one FFmpeg process
regardless of the number of VRChat listeners.

HLS is a compatibility alternative, **not a zero-buffering guarantee**.
Expect additional initial buffering and some seconds of delay; VRChat AVPro on
PC has known reports of significant HLS latency.

## Enable for a station

In `config.toml`:

```toml
[stations.neffex]
name = "NEFFEX Radio"
genre = "Rock"
path = "/music/neffex"
vrchat_hls = true
```

Restart the server; check that FFmpeg has `libx264` and `aac` encoders.
The Docker image includes FFmpeg. A writable persistent `/data` directory
is required to generate HLS files at `/data/vrchat-hls/neffex`.

- Existing MP3: `https://radio.nekosunevr.co.uk/neffex/stream`
- New VRChat HLS: `https://radio.nekosunevr.co.uk/neffex/vrchat/index.m3u8`
- 4-second target HLS segments; six live segments retained in the playlist

The HLS playlist may initially respond with HTTP 503 ("warming up") until
FFmpeg generates its first media segment. Segment URLs are relative within
the HLS manifest; clients should follow them rather than construct their own.

## Public GET radio API

This is a read-only JSON API, with cross-origin requests allowed and **no API
key required**. It does not expose listener IPs, credential secrets, music
source URLs or file locations.

```http
GET /api/v1/radios
GET /api/v1/radios?online_only=true
GET /api/v1/radios/neffex
```

Example list response (illustrative, not live server data):

```json
{
  "version": 1,
  "updated_at": "2026-11-04T20:00:00Z",
  "count": 1,
  "radios": [
    {
      "id": "neffex",
      "name": "NEFFEX Radio",
      "description": "Rock music",
      "genre": "Rock",
      "online": true,
      "listeners": 3,
      "track_count": 25,
      "stream_url": "https://radio.nekosunevr.co.uk/neffex/stream",
      "vrchat_hls_url": "https://radio.nekosunevr.co.uk/neffex/vrchat/index.m3u8",
      "preferred_url": "https://radio.nekosunevr.co.uk/neffex/vrchat/index.m3u8",
      "format": "hls",
      "artwork_url": "https://radio.nekosunevr.co.uk/logo.svg",
      "now_playing": {
        "title": "Example Track",
        "artist": "Example Artist",
        "album": ""
      }
    }
  ]
}
```

`format` describes the `preferred_url`: either `hls` or `mp3`. The
`vrchat_hls_url` field is **omitted** unless HLS is configured. Consumers
should use `preferred_url` for VRChat AVPro, or select `stream_url` for
MP3-capable players. Stations marked offline stay in the unfiltered list but
are omitted by `online_only=true`. `updated_at` is the response generation
time. Station order follows `config.toml`.

VRChat UdonSharp typically fetches JSON with
`VRCStringDownloader.LoadUrl` and parses with `VRCJson`. Select the
`preferred_url` for the station you want to play, and provide it to the
AVPro player through the VRChat video player API. Ensure the world is allowed
to load your custom HTTPS host. Poll the list moderately (for example every
30-60 seconds) rather than per frame.

## URL restrictions in VRChat

`radio.nekosunevr.co.uk` is a custom domain, not automatically on VRChat's
video host allowlist. Configure **Video Player Allowed Domains** for the
world as required, and viewers may need the *Allow Untrusted URLs* setting.
VRChat's `Allow Untrusted URLs` rules and public-instance restrictions vary;
review the current official VRChat documentation.

## Reverse proxy diagnostics

If buffering persists after deploying the new build, compare the source
directly to the Cloudflare/proxy path; test at least 30 seconds of continuous
audio (do not paste credentials, cookies or Cloudflare tokens).

```bash
curl -sS -D /tmp/radio-headers.txt --max-time 30 \
  -o /tmp/radio-sample.mp3 \
  https://radio.nekosunevr.co.uk/neffex/stream
cat /tmp/radio-headers.txt
ffprobe -hide_banner -v warning /tmp/radio-sample.mp3

curl -i https://radio.nekosunevr.co.uk/api/v1/radios
curl -i https://radio.nekosunevr.co.uk/neffex/vrchat/index.m3u8
```

Check that the MP3 stream responds `200` with `audio/mpeg`; that a
successful manifest responds `200` with `application/vnd.apple.mpegurl`;
that its referenced TS segments load with `200` and `video/mp2t`; and that
the proxy does not cache playlists, compress MP3, or buffer streaming origin
responses. For nginx use `proxy_buffering off;` and
`proxy_read_timeout 3600s;`. Consider bypassing Cloudflare's HTTP proxy for
the **audio-only** host if real-time delivery is disrupted, but keep HTTPS
and access controls in place.

## Compatibility and performance notes

- HLS may use more storage and CPU, with a shared H.264/AAC transcode per
  station. Monitor resource usage when enabling many stations.
- Keep all client-facing URLs HTTPS for cross-platform support.
- This is not a game-client authentication API; it exposes public station
  discovery only. Private admin controls remain authenticated.
- Test actual VRChat playback on PC and Quest before claiming it is fixed.
