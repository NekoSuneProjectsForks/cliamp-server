package server

import (
 "html/template"
 "net/http"
)

var musicLibraryPage = template.Must(template.New("music-library").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>{{.Name}} · Music Library</title>
<style>
:root{color-scheme:dark;--bg:#080f0c;--panel:#112019;--border:#2a4939;--mint:#6be4a7;--muted:#a6c0b3}
*{box-sizing:border-box}body{margin:0;font:15px/1.5 system-ui,sans-serif;background:radial-gradient(ellipse at 20% 0,#143c27,transparent 50%),var(--bg);color:#f1fff6}
header,main{max-width:1020px;margin:auto;padding:22px}header{display:flex;justify-content:space-between;gap:12px;flex-wrap:wrap;align-items:center;border-bottom:1px solid var(--border)}
a{color:var(--mint)}h1{font-size:clamp(27px,5vw,46px);margin:16px 0 4px}p,.muted{color:var(--muted)}
.panel{border:1px solid var(--border);border-radius:15px;background:var(--panel);padding:18px;margin:16px 0}
button,.button{font:inherit;display:inline-block;background:#21573a;color:white;padding:10px 15px;border:1px solid #3b8b5c;border-radius:9px;cursor:pointer;text-decoration:none}
button:hover,.button:hover{background:#2a6d49}button:disabled{opacity:.45;cursor:not-allowed}
input{font:inherit;background:#07120c;color:white;border:1px solid var(--border);border-radius:8px;padding:11px;width:100%}
#tracks{list-style:none;padding:0;margin:0}#tracks li{display:flex;align-items:center;justify-content:space-between;gap:10px;padding:12px 3px;border-bottom:1px solid var(--border)}#tracks li strong{display:block}#tracks li small{color:var(--muted)}
#tracks li button{flex:none}.row{display:flex;gap:8px;flex-wrap:wrap;align-items:center}audio{width:100%;margin-top:13px}
#cover{width:98px;height:98px;border-radius:11px;object-fit:cover;background:#102a20}
.current{display:flex;gap:15px;align-items:center}.current div{min-width:0;overflow-wrap:anywhere}
</style></head>
<body data-station="{{.ID}}">
<header><strong>Neko<span style="color:var(--mint)">Sune</span> Radio · {{.Name}}</strong>
<nav><a href="/">Stations</a> · <a href="/{{.ID}}/stream.m3u">Live radio</a> · <a href="/api/{{.ID}}/playlist.m3u">Download playlist</a></nav></header>
<main>
<p style="color:var(--mint);font-weight:700;letter-spacing:2px">ON-DEMAND MUSIC</p>
<h1>{{.Name}} Playlist</h1>
<p>Play individual songs from this station without joining the live radio broadcast. YouTube items are delivered through the server as MP3 on demand, without permanent downloads.</p>
<section class="panel"><div class="current"><img id="cover" alt="Current station artwork" src="/api/{{.ID}}/art.png">
<div><strong id="current">Choose a song</strong><p id="status">Music library loading…</p></div></div>
<audio id="audio" preload="none" controls></audio>
<div class="row" style="margin-top:12px"><button id="prev">Previous</button><button id="next">Next</button><button id="shuffle" aria-pressed="false">Shuffle off</button><button id="live">Play live radio</button></div>
</section>
<section class="panel"><label for="query">Search music library</label><input id="query" placeholder="Filter by artist, title, album or source" autocomplete="off">
<p id="count">Loading…</p><ul id="tracks"></ul></section>
</main>
<script>
'use strict';
const station=document.body.dataset.station;
const $=id=>document.getElementById(id);
const audio=$('audio');let tracks=[],index=-1,shuffle=false;
const safe=String;
function render(){
 const filter=$('query').value.trim().toLowerCase();
 const matches=tracks.map((song,i)=>({song,i})).filter(({song})=>
   [song.title,song.artist,song.album,song.source].join(' ').toLowerCase().includes(filter));
 $('tracks').replaceChildren();
 $('count').textContent=matches.length+' of '+tracks.length+' tracks';
 for(const {song,i} of matches){
  const li=document.createElement('li'), info=document.createElement('div');
  const label=document.createElement('strong');label.textContent=(song.artist? song.artist+' — ':'')+song.title;
  const sub=document.createElement('small');sub.textContent=[song.album,song.source].filter(Boolean).join(' · ');
  info.append(label,sub);
  const b=document.createElement('button');b.type='button';b.textContent=index===i?'Playing':'Play';b.onclick=()=>play(i);
  li.append(info,b);$('tracks').append(li);
 }
}
async function play(i){
 if(i<0||i>=tracks.length)return;
 index=i;const song=tracks[i];
 $('current').textContent=(song.artist?song.artist+' — ':'')+song.title;
 $('status').textContent='Streaming '+song.source+' via NekoSune Radio';
 $('cover').src=song.cover_url||'/api/'+encodeURIComponent(station)+'/art.png';
 audio.src=song.stream_url||song.url;
 try{await audio.play()}catch(e){$('status').textContent='Tap play to start audio'}
 render();
}
$('query').addEventListener('input',render);
$('next').onclick=()=>{
 if(!tracks.length)return;
 play(shuffle?Math.floor(Math.random()*tracks.length):(index+1)%tracks.length)
};
$('prev').onclick=()=>play(index>0?index-1:Math.max(0,tracks.length-1));
$('shuffle').onclick=()=>{
 shuffle=!shuffle;$('shuffle').textContent=shuffle?'Shuffle on':'Shuffle off';$('shuffle').setAttribute('aria-pressed',String(shuffle))
};
$('live').onclick=()=>{index=-1;$('cover').src='/api/'+encodeURIComponent(station)+'/art.png';audio.src='/'+encodeURIComponent(station)+'/stream';audio.play().catch(()=>{});$('current').textContent='Live radio';$('status').textContent='Live station broadcast';render()};
audio.addEventListener('ended',()=>{$('next').click()});
audio.addEventListener('error',()=>{$('status').textContent='Track unavailable or failed to start. Try another song.'});
(async()=>{try{
 const r=await fetch('/api/'+encodeURIComponent(station)+'/playlist',{cache:'no-store'});
 if(!r.ok)throw Error('Library unavailable (check station expose_tracks setting)');
 const data=await r.json();tracks=data.tracks||[];$('status').textContent='Ready to play';render()
 }catch(e){$('status').textContent=e.message;$('count').textContent='Unable to load playlist'}})();
</script></body></html>`))

func serveMusicLibraryPage(w http.ResponseWriter,r *http.Request,id,name string){
 w.Header().Set("Content-Type","text/html; charset=utf-8")
 w.Header().Set("Cache-Control","no-store")
 w.Header().Set("X-Content-Type-Options","nosniff")
 _=musicLibraryPage.Execute(w,struct{ID,Name string}{ID:id,Name:name})
}
