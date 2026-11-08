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

## FFmpeg-only visual radio backgrounds (no browser engine)

The **Admin → VRChat visuals** page lets operators configure a station's
video presentation without affecting its existing radio audio. Only enabled
VRChat HLS stations have a live HLS output. This feature does not launch
Chromium, Puppeteer, Electron or a browser. A lightweight Go frame feeder
assembles **one 320 × 180 RGB frame per second**; FFmpeg handles MP4
decoding and H.264/AAC HLS encoding, and the drawtext filter hot-reloads
metadata from a UTF-8 file. The HTML admin page is only a configuration UI.

**Available controls**

- Solid dark background (lowest CPU; default).
- Upload JPEG or PNG as a still background, or replace it while broadcasting.
- Upload a local MP4 video (max 32 MiB) to play in an infinite loop. An
  independent FFmpeg process decodes it at 1 FPS; changing the MP4 restarts
  *only* that background decoder, not the main radio or HLS muxer.
- Upload a fallback radio cover and/or a cover for the currently playing song.
  Current-song covers are saved by normalised artist and title and reused
  when the same song plays again.
- Show/hide the album cover, show/hide the current title/artist, or use manual
  text overrides in the admin dashboard.
- Changes are stored under `/data/vrchat-assets/<station>` and are loaded
  on the next 1 FPS visual frame. Updates do not restart the running encoder.
- When the station is disabled in the admin dashboard, the HLS video
  encoder is cancelled, the manifest/segments are unavailable via HTTP,
  and the game API reports offline. Re-enabling restarts HLS; initial
  segments need a few seconds to be generated.

### Keep the underlying audio alive

Visual updates do **not** alter `/<id>/stream`, the existing audio
playlist, the station music choice, or the existing FFmpeg radio encoder.
Only the video representation is recomposed. The HLS output uses a single
shared FFmpeg video/audio muxer per station and a small low-FPS video decoder
only when MP4 mode is selected.

Now-playing text updates from the station's current playback metadata. This
does not change the song itself. Per-track cover art is currently **manually
uploaded**, not automatically downloaded from iTunes; the default cover is
used when no per-track image has been provided.

### Admin JSON and upload API (Bearer authentication)

```http
GET    /api/admin/stations/neffex/visuals
PUT    /api/admin/stations/neffex/visuals
POST   /api/admin/stations/neffex/visuals/upload
DELETE /api/admin/stations/neffex/visuals/{background|video|cover|track-cover}
```

PUT JSON example:

```json
{
  "mode": "image",
  "show_cover": true,
  "show_title": true,
  "title_override": "",
  "artist_override": ""
}
```

The multipart POST upload accepts `kind` and `file` fields. `kind` is
`background`, `video`, `cover` or `track-cover`. Image uploads are
validated and re-encoded to PNG; videos are limited to 32 MiB and checked by
ffprobe for a video stream. Admin credentials must never appear in client-side
VRChat code.

### Raspberry Pi 5 recommendations

Begin with a single HLS station at **320×180, 1 FPS, x264 ultrafast,
two encoder threads, 96 kbps AAC audio**, using an image background.
HLS at this frame rate is designed to transmit artwork and metadata, **not
fluid 30 FPS music video**. MP4 mode decodes at the same 1 FPS, reducing CPU
cost but with visibly stepped animation. Monitor CPU, temperature and total
concurrent HLS station count before enabling additional stations. While
disabled, stations don't run an HLS FFmpeg process.

The HLS muxer is retained across title, cover, still image or MP4 changes.
If the network audio source disconnects or FFmpeg fails, the supervisor
starts a new HLS generation, which can cause a temporary interruption to
HLS playback. This feature does not guarantee zero buffering in VRChat;
test it with AVPro on the target platform and keep caching disabled for
`index.m3u8` requests.


## Automatic current-song art.png

Each HLS-enabled station maintains its current cover at:

```text
/data/vrchat-assets/<station-id>/art.png
/data/vrchat-assets/<station-id>/nowplaying.txt
```

The live image can be retrieved by clients with:

```http
GET /<station-id>/art.png
```

For example, `https://radio.nekosunevr.co.uk/neffex/art.png`.

The one-frame-per-second HLS Go frame feeder updates `art.png` from files
already on the Raspberry Pi, without making iTunes/yt-dlp HTTP requests. A background station-artwork worker also synchronizes the same local `art.png` every five seconds when HLS is disabled, so the public artwork API and HLS visuals share one file.
The background YouTube metadata worker and the new radio-artwork service reuse previously saved per-track PNGs. The radio-artwork service looks up artwork only for uncached songs, caches matching iTunes art as PNG, and throttles missing covers; no HTTP request or iTunes lookup occurs in the HLS frame loop or on public API reads.

Selection priority is:
1. Saved per-song cover (manual upload or imported iTunes result for the current artist/title)
2. Station fallback cover (`cover.png`)
3. Generated local CD-in-a-jewel-case fallback (`fallback.png`), so `art.png` always has a valid image after the artwork service starts

A song change or image upload/replacement is detected on the next HLS
visual frame, **without restarting MP3 audio or the continuous HLS encoder**.
Unchanged files are not rewritten every second, preserving Pi 5 storage.
The public image response has `Cache-Control: no-store` so browsers and
proxies can fetch the updated image. The admin **VRChat visuals** page
shows the current cover and has a **Copy art.png URL** button next to
the MP3 and HLS copy controls.

The `/data` Docker volume must be persistent. The actual audio and complete
artist/title remain independent of the artwork file. The current art is
updated while HLS is running; an HLS-disabled/offline station does not
actively refresh the generated file.

### Reusable artwork URL for all radio consumers

`/api/<station>/art.png` serves only the locally cached `art.png` for that station, or an in-memory generated CD jewel case if the local file is not ready. `/api/public/radio`, `/api/<station>/nowplaying`, and `/api/v1/radios` all refer to the same local cover URL. The existing `/<station>/art.png` endpoint and HLS video reuse the exact same cache. Cached images are persisted under `/data/vrchat-assets/<station>/` with the Docker data volume, so a song you've already played does not need another external album-cover download on restart.
