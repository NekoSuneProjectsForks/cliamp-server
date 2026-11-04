# Asynchronous YouTube metadata refresh

The CLIAMP admin server no longer executes `yt-dlp` inside HTTP handlers.
A valid video can be added and the **Refresh YouTube title** button can be
pressed without waiting on YouTube, so a slow lookup cannot exhaust the
Cloudflare HTTP request's time budget.

## Request flow

- `POST /api/admin/stations/{id}/youtube` persists the URL and immediately
  returns HTTP **202 Accepted** with a queued status.
- `POST /api/admin/stations/{id}/youtube/metadata` now queues a refresh and
  immediately returns HTTP **202 Accepted**, rather than invoking yt-dlp in
  the HTTP request.
- `GET /api/admin/stations/{id}/youtube/metadata/status?url=...` reports
  `idle`, `queued`, `running`, `completed` or `failed`.
- The dashboard checks status periodically and refreshes the Music Library
  when the job completes. Closing the page does not cancel a queued job.
- Existing YouTube links without saved metadata are queued automatically on
  server startup, with no incoming browser request needed.

All endpoints remain admin-authenticated. The status route returns no
YouTube cookies or origin credentials.

## Raspberry Pi stability

Only **one yt-dlp metadata process** runs at once, with a strict 15-second
lookup deadline. The bounded 512-job queue prevents unbounded spawning and
duplicate queued/running work is coalesced. Tasks are in-memory: a restart
drops incomplete work, but cached results survive in
`/data/track-metadata.json`; uncached links are queued again on startup.
The active radio audio encoder and the HLS video encoder are not restarted
when metadata resolves.

A failed lookup is reported in job status without causing the radio server
to return HTTP 502. Users may retry manually. Errors from YouTube can still
occur due to upstream restrictions, expired audio URLs, broken yt-dlp
installations or connectivity; background handling isolates the admin HTTP
request, but does not guarantee yt-dlp playback success.

## Cloudflare 502 investigation

A Cloudflare error page showing **Browser Working**, **Cloudflare Working**
and **Host Error** identifies an upstream/origin problem, but does not by
itself prove that this handler caused the failure. Check:

```bash
docker compose ps
docker compose logs --tail=200 cliamp
docker stats --no-stream
curl -i --max-time 5 http://127.0.0.1:8000/api/v1/radios
curl -i --max-time 10 https://radio.nekosunevr.co.uk/api/v1/radios
```

If the local API fails, inspect the server/container, Docker health, and Pi
CPU/RAM/OOM logs. If local works and the public hostname fails, inspect
Cloudflare Tunnel or nginx upstream settings and timeouts. The 502 may also
reflect a crashed origin or network failure; the change here removes one
known source of long-running HTTP requests.

Rebuild/redeploy the server image to enable these changes. Existing browsers
may need a hard refresh for the updated admin JavaScript.

## ICY now-playing and YouTube availability

Clients sending `Icy-MetaData: 1` to `/<station>/stream` receive
`icy-metaint` and a correctly aligned `StreamTitle` metadata block.
A YouTube track with cached metadata produces, for example:

```text
StreamTitle='THIRST - MAYHEM';
```

The MP3 byte stream and audio connections are not restarted. When yt-dlp
metadata arrives **while a track is already playing**, both the current
track and matching retained ring metadata events are updated in place.
The ICY writer automatically emits the new title at its next metadata block;
the same Now Playing values are used by the game API and HLS graphics.

A video that has not been resolved yet shows
`StreamTitle='YouTube metadata pending';` instead of a raw watch URL.
The original video URL stays internal for decoding and library management.

### Dead video safeguards

The dashboard distinguishes these conditions:

- `private`: **Cannot play — private video.** Keep the URL but omit it
  from the playable rotation. Recheck automatically; a later public result
  restores it.
- `unlisted`: URL is retained and **remains playable** if accessible.
  Unlisted does not mean private or deleted.
- `unknown`: temporary YouTube failures, 403/429, missing cookies,
  bot checks, rate limits or timeouts. **Never automatically remove.**
- `deleted`: only an explicit yt-dlp message that the **uploader removed
  the video** is considered deletion evidence. The link is excluded from
  playback, but remains saved during verification.

For automatic permanent removal, the system requires **three matching
uploader-deleted responses separated by at least 24 hours**, spanning
at least 48 hours. A periodic worker retries private/deleted entries
with only one yt-dlp process at a time. Any different result resets the
deletion confirmation count. The evidence is saved in
`/data/youtube-video-health.json`. No other failure category authorizes
automatic deletion. Removing a link never terminates current radio audio;
the next playlist transition simply uses the remaining playable tracks.

Manual deletion in Manage Music is still available, with confirmation.
If all links are unavailable, the station falls back to silence until
a playable source returns. No automatic deletion applies to an unlisted
video, even if it cannot be fetched temporarily.
