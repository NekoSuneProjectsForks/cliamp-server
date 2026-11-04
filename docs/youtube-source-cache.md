# YouTube title, artist and artwork cache (Raspberry Pi friendly)

CLIAMP's yt-dlp metadata resolver now checks a persistent **SQLite**
database before invoking the extractor. The cache is keyed by the
11-character YouTube video ID (e.g. `AMmBY2D9IJU`), so these links share
one metadata record:

```
https://www.youtube.com/watch?v=AMmBY2D9IJU
https://youtu.be/AMmBY2D9IJU
https://music.youtube.com/watch?v=AMmBY2D9IJU
```

Data is saved at `/data/youtube-source-cache.sqlite`. It contains the
cleaned artist, cleaned title, original YouTube title, album when available,
artwork lookup status and the matched artwork URL.

The original station-specific `/data/track-metadata.json` remains for
manual overrides, which always take precedence over the shared cache.
Cached metadata is applied as playlists load, without blocking startup.
The background worker imports the cached artist/title into any additional
station using the same video ID; **no second yt-dlp request** is made.

## Album covers without API keys

For new YouTube video IDs only, the resolver tries the public **iTunes Search
API** with the cleaned artist/title (not the raw promotional title).
It requires a matching artist and song rather than blindly taking the first
search result. If a match exists, the artwork is fetched once from an approved
Apple artwork host and saved under `/data/youtube-artwork/<video-id>.png`.
The resulting image is applied to the matching HLS song cover.

The SQLite cache remembers both successful matches and missing artwork,
avoiding repeated requests after each reboot or when the same video is
used on several stations. Existing manually uploaded artwork is **never
overwritten** by the automatic artwork importer.

Cover matching is best-effort: an artist/title may not have an Apple Music
catalog entry. In that case, the fallback station cover or a manually uploaded
song cover is used. This feature does not scrape or download arbitrary album
artwork, nor make external cover requests while a song is playing.

## VRChat HLS admin shortcuts

In **Admin → VRChat visuals**, choose a radio and use the two read-only URL
fields with **Copy MP3 URL** and **Copy HLS URL**. HLS copy is disabled if
the station has not enabled its optional VRChat HLS encoder.

For example:

```
https://radio.nekosunevr.co.uk/neffex/stream
https://radio.nekosunevr.co.uk/neffex/vrchat/index.m3u8
```

## Titles that fit within the video

The FFmpeg HLS video remains at 320x180/1 FPS for Pi 5 stability.
Now-playing text is truncated to a conservative ~30-glyph budget, with an
ellipsis for long artist/title strings. Wide glyphs count extra; short titles
remain unchanged. The actual ICY title, metadata database, MP3 stream,
current song and API metadata retain their **full** artist/title; only the
drawn HLS text is shortened.

## Source and network scope

This cache is **YouTube-only** for now, because the current active yt-dlp
station source adapter is YouTube. The schema and station abstraction can
be extended to SoundCloud, Bandcamp and other supported extractor types
when their station adapters exist. Local files, S3 and direct HTTPS sources
are not sent to yt-dlp or iTunes by this change.

Do not delete the Docker `/data` volume during upgrades. The SQLite DB and
artwork directory are both required to retain metadata and images between
restarts. Real YouTube extraction and iTunes cover matches depend on external
service availability and need a production smoke test.
