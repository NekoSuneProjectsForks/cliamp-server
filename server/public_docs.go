package server

const publicDocsHTML = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>API Documentation | NekoSune Radio</title><style>
:root{color-scheme:dark}*{box-sizing:border-box}body{background:radial-gradient(ellipse at top left,#163b2b,#07100b 55%);color:#f1fff7;font:15px/1.65 system-ui,Segoe UI,sans-serif;margin:0}header,main{max-width:1050px;padding:22px;margin:auto}header{display:flex;justify-content:space-between;align-items:center;border-bottom:1px solid #30533d}a{color:#74e7ad}nav a{margin-left:20px}h1{font-size:clamp(32px,5vw,52px);line-height:1.15}h2{margin-top:36px;color:#8ef2b9}p{color:#acc4b5}.pill{color:#8ef2b9;font-weight:bold;font-size:12px}section{background:#101b15;border:1px solid #294d36;border-radius:14px;padding:22px;margin:18px 0}code,pre{font-family:ui-monospace,Consolas,monospace}code{color:#99f4bc}pre{padding:16px;background:#070f0b;border-radius:9px;overflow-x:auto;white-space:pre}table{width:100%;border-collapse:collapse}th,td{text-align:left;padding:12px;border-bottom:1px solid #28402f}th{color:#8ef2b9}button{border:1px solid #44795d;background:#1d422c;color:white;border-radius:7px;padding:8px 12px;cursor:pointer}button:hover{background:#2d6342}.muted{color:#a8b9ae}.inline{display:flex;gap:9px;align-items:center;flex-wrap:wrap}.copy{margin-left:auto}</style></head><body>
<header><strong>NekoSune <span style="color:#8ef2b9">Radio / Developers</span></strong><nav><a href="/">Listen</a><a href="/docs">API docs</a><a href="/admin">Admin</a></nav></header><main>
<p class="pill">PUBLIC REST + STREAMING API</p><h1>Build with NekoSune Radio.</h1><p>Integrate our radio streams, live song information, and anonymised listener statistics into your apps, bots, and websites. All endpoints below are public and require no API key.</p>
<section><h2>Base URL</h2><pre>https://radio.nekosunevr.co.uk</pre><p class="muted">Use HTTPS. Availability depends on the station configuration; some optional endpoints return HTTP 404 when disabled.</p></section>
<section><h2>Public endpoints</h2><table><thead><tr><th>Method</th><th>Endpoint</th><th>Purpose</th></tr></thead><tbody>
<tr><td>GET</td><td><a href="/api/public/radio"><code>/api/public/radio</code></a></td><td>Station names, listeners, now playing, track totals and online status</td></tr>
<tr><td>GET</td><td><a href="/stations"><code>/stations</code></a></td><td>Station directory and streaming URLs</td></tr>
<tr><td>GET</td><td><a href="/statistics"><code>/statistics</code></a></td><td>Historical aggregated listeners, countries and sessions (requires stats)</td></tr>
<tr><td>GET</td><td><a href="/neffex/statistics"><code>/{id}/statistics</code></a></td><td>Historical metrics for a single station (requires stats)</td></tr>
<tr><td>GET</td><td><a href="/streams.m3u"><code>/streams.m3u</code></a></td><td>Playlist of all radio streams</td></tr>
<tr><td>GET</td><td><a href="/streams.pls"><code>/streams.pls</code></a></td><td>PLS playlist of all radio streams</td></tr>
<tr><td>GET</td><td><code>/{id}/stream</code></td><td>Continuous MP3 radio audio with ICY headers</td></tr>
<tr><td>GET</td><td><code>/{id}/stream.m3u</code></td><td>Station M3U playlist</td></tr>
<tr><td>GET</td><td><code>/{id}/stream.pls</code></td><td>Station PLS playlist</td></tr>
<tr><td>GET</td><td><code>/{id}/tracks</code></td><td>Track catalogue only if enabled per station</td></tr>
<tr><td>GET</td><td><code>/{id}/tracks/statistics</code></td><td>Aggregated plays if tracks and stats are enabled</td></tr>
</tbody></table></section>
<section><h2>Live radio JSON</h2><div class="inline"><span class="pill">GET /api/public/radio</span><button class="copy" data-copy="curl https://radio.nekosunevr.co.uk/api/public/radio">Copy curl</button></div><p>The API returns live station information without disclosing listener IP addresses or identities.</p><pre>{
  "stations": [
    {
      "id": "neffex",
      "name": "Neffex Station",
      "description": "",
      "genre": "Music",
      "listeners": 3,
      "tracks": 40,
      "online": true,
      "current_track": {
        "title": "Example track",
        "artist": "Example artist",
        "album": "Example album"
      },
      "stream": "/neffex/stream"
    }
  ],
  "total_listeners": 3
}</pre><p class="muted">Sample values only. These change in real time.</p></section>
<section><h2>Fetch the current song (JavaScript)</h2><div class="inline"><button class="copy" data-copy="fetch('https://radio.nekosunevr.co.uk/api/public/radio').then(r => r.json()).then(data => console.log(data.stations))">Copy example</button></div><pre>const response = await fetch(
  "https://radio.nekosunevr.co.uk/api/public/radio"
);
if (!response.ok) throw new Error("Radio unavailable");
const data = await response.json();
const neffex = data.stations.find(s => s.id === "neffex");
console.log(neffex?.current_track.title);
console.log(neffex?.listeners);</pre><p>Poll sensibly, such as every 10–30 seconds. Do not request the API every frame.</p></section>
<section><h2>Aggregated statistics</h2><p class="pill">GET /statistics</p><pre>curl https://radio.nekosunevr.co.uk/statistics</pre><p>Statistics require the server administrator to configure <code>[stats] db_path</code>. Country/city values additionally require <code>[geo] db_path</code> and completed listener sessions. These endpoints expose aggregated values, not raw listener addresses.</p></section>
<section><h2>Stream audio and metadata</h2><pre>https://radio.nekosunevr.co.uk/neffex/stream
https://radio.nekosunevr.co.uk/nekoryza/stream</pre><p>Use the stream URL in a standard HTML audio player. Clients that support ICY metadata may set the <code>Icy-MetaData: 1</code> request header to receive song title updates.</p></section>
<section><h2>Privacy and administration</h2><p>Individual listener IP addresses, the ban list and mutation endpoints are deliberately excluded from the public API. Admin access requires a separate Bearer credential. Never publish your admin token in frontend code or a public repository.</p><p><a href="https://github.com/NekoSuneProjectsForks/cliamp-server/blob/main/docs/api.md">More technical API information on GitHub</a></p></section>
</main><script>document.querySelectorAll("[data-copy]").forEach(b=>b.addEventListener("click",async()=>{try{await navigator.clipboard.writeText(b.dataset.copy);b.textContent="Copied!"}catch(e){b.textContent="Copy unavailable"}}));</script></body></html>`
