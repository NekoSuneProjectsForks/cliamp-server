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
