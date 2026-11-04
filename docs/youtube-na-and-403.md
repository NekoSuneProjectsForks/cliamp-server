# yt-dlp metadata NA and YouTube audio 403 fixes

## Missing optional metadata (bare NA)

Some YouTube videos have no separate `artist`, `track`, `album` or
`creator` field. yt-dlp formats a missing `%(field)j` as **bare `NA`**,
not the JSON string `"NA"`. The old parser tried JSON decoding and failed
with:

```text
invalid yt-dlp metadata field 1: invalid character 'N'
```

The parser now accepts bare `NA` and `null` as missing optional values,
retains JSON parsing for provided values, and continues extracting
`Artist - Title` from the cleaned YouTube video title. It does not overwrite
existing cached/manual titles on an ordinary refresh.

## FFmpeg signed YouTube URL 403

Previously the track decoder asked `yt-dlp --get-url` for a signed
`googlevideo.com/videoplayback?... ` link and handed that URL directly
to FFmpeg. YouTube may require special request headers or a particular
client state; FFmpeg did not necessarily use yt-dlp's request headers and
could get HTTP **403 Forbidden** even when metadata lookup succeeded.

The decoder now uses a streaming pipe:

```text
YouTube video URL
    -> yt-dlp -f bestaudio/best -o -  (provider-aware request headers)
    -> FFmpeg -i pipe:0             (PCM decoder)
    -> existing continuous MP3 radio encoder
```

The signed googlevideo URL is never passed through FFmpeg command
arguments. The radio still uses a single transcoder per active track and
one shared MP3 encoder per station, **not one video decoder per listener**.
The yt-dlp subprocess is reaped when FFmpeg exits; stdout is streamed, not
cached into memory or to a large temporary file.

### If a 403 continues

A 403 inside yt-dlp itself may indicate YouTube sign-in restrictions,
bot checks, IP restrictions, player challenges, unavailable formats, or
other upstream conditions. This patch cannot bypass those restrictions.
Do not treat a 403 as permanent deletion, and do not automatically remove
the URL.

Check that Docker is running the newly built image and that the installed
yt-dlp is current. Test manually on the Raspberry Pi (only with media
you have permission to access/broadcast):

```bash
docker compose exec cliamp yt-dlp \
  --ignore-config --no-playlist --js-runtimes node \
  -f 'bestaudio/best' -o /dev/null \
  'https://www.youtube.com/watch?v=AMmBY2D9IJU'
```

This downloads only to `/dev/null` for testing, without keeping media.
Do not post output containing signed URLs, cookies or other tokens.
If command-line yt-dlp also returns 403, the upstream request is failing;
if yt-dlp succeeds but the radio doesn't, inspect the source-to-FFmpeg
pipe and FFmpeg codec support.

## Deployment and safety

Rebuild/redeploy the server image for the changes to take effect.
No cached artist/title or artwork records need deleting: previously failed
items can be requeued and their successful metadata will be persisted.
The existing safeguards for private, unlisted and ambiguously unavailable
videos remain in place.

CI includes a local fake yt-dlp/FFmpeg test that proves the pipe handoff
without reaching YouTube. Real playback on the Raspberry Pi and actual
VRChat listening still need integration testing.
