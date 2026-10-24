package server

const radioGlobeJS = `(function(){
"use strict";
const canvas=document.getElementById("globe");if(!canvas)return;
const ctx=canvas.getContext("2d");if(!ctx)return;
const list=document.getElementById("countries"),err=document.getElementById("stats-err");
const W=720,H=460,R=172,CX=W/2,CY=H/2+4,rad=Math.PI/180;
let yaw=-20*rad,pitch=10*rad,drag=false,downX=0,downY=0,moved=false;
let world=[],countries=[],tooltip=null,last=0;
const reduced=window.matchMedia("(prefers-reduced-motion: reduce)").matches;
const aliases={"United States":"United States of America","UK":"United Kingdom","Russia":"Russian Federation","South Korea":"South Korea","Czechia":"Czech Republic"};
function norm(s){return (s||"").toLowerCase().normalize("NFD").replace(/[\u0300-\u036f]/g,"").replace(/[^a-z]/g,"")}
function position(lon,lat){
 const l=lon*rad+yaw,p=lat*rad;
 const z=Math.cos(p)*Math.cos(l)*Math.cos(pitch)+Math.sin(p)*Math.sin(pitch);
 const x=Math.cos(p)*Math.sin(l);
 const y=Math.sin(p)*Math.cos(pitch)-Math.cos(p)*Math.cos(l)*Math.sin(pitch);
 return {x:CX+R*x,y:CY-R*y,z};
}
function drawPolyline(ring){
 let started=false,prev=null;
 for(let i=0;i<ring.length;i+=Math.max(1,Math.floor(ring.length/100))){
 const p=position(ring[i][0],ring[i][1]);
 if(p.z>0&&(!prev||prev.z>0&&Math.hypot(p.x-prev.x,p.y-prev.y)<70)){
 if(!started){ctx.moveTo(p.x,p.y);started=true}else ctx.lineTo(p.x,p.y);
 }else started=false;
 prev=p;
 }
}
function draw(t){
 ctx.clearRect(0,0,W,H);
 const halo=ctx.createRadialGradient(CX,CY,R*.7,CX,CY,R*1.28);
 halo.addColorStop(0,"rgba(28,184,104,.13)");halo.addColorStop(1,"transparent");
 ctx.fillStyle=halo;ctx.fillRect(0,0,W,H);
 const sphere=ctx.createRadialGradient(CX-R*.3,CY-R*.4,R*.03,CX,CY,R);
 sphere.addColorStop(0,"#214d44");sphere.addColorStop(.75,"#102927");sphere.addColorStop(1,"#071817");
 ctx.beginPath();ctx.arc(CX,CY,R,0,Math.PI*2);ctx.fillStyle=sphere;ctx.fill();
 ctx.save();ctx.beginPath();ctx.arc(CX,CY,R,0,Math.PI*2);ctx.clip();
 ctx.strokeStyle="rgba(116,230,179,.14)";ctx.lineWidth=.65;
 ctx.beginPath();
 for(let lat=-75;lat<=75;lat+=15){let started=false;for(let lon=-180;lon<=180;lon+=5){const p=position(lon,lat);if(p.z<=0){started=false;continue}if(!started){ctx.moveTo(p.x,p.y);started=true}else ctx.lineTo(p.x,p.y)}}
 for(let lon=-180;lon<180;lon+=20){let started=false;for(let lat=-90;lat<=90;lat+=4){const p=position(lon,lat);if(p.z<=0){started=false;continue}if(!started){ctx.moveTo(p.x,p.y);started=true}else ctx.lineTo(p.x,p.y)}}
 ctx.stroke();
 ctx.strokeStyle="rgba(99,206,157,.46)";ctx.lineWidth=.85;ctx.beginPath();
 for(const ring of world)drawPolyline(ring);
 ctx.stroke();
 const visible=[];
 for(const c of countries){
 if(!Number.isFinite(c.lat)||!Number.isFinite(c.lon))continue;
 const p=position(c.lon,c.lat);if(p.z<=0)continue;
 const radius=Math.min(8,3.4+Math.sqrt(c.count)*1.2);
 const pulse=1+(.16*Math.sin(t/420+c.lon));
 ctx.beginPath();ctx.arc(p.x,p.y,radius*2.3*pulse,0,Math.PI*2);ctx.fillStyle="rgba(111,255,165,.12)";ctx.fill();
 ctx.beginPath();ctx.arc(p.x,p.y,radius,0,Math.PI*2);ctx.fillStyle="#8bffc1";ctx.fill();
 visible.push({...c,x:p.x,y:p.y,radius});
 }
 ctx.restore();
 ctx.beginPath();ctx.arc(CX,CY,R,0,Math.PI*2);ctx.strokeStyle="rgba(121,229,169,.4)";ctx.lineWidth=1.5;ctx.stroke();
 if(tooltip){const hovered=visible.find(c=>c.name===tooltip.name);if(hovered){const label=hovered.name+": "+hovered.count+" listening";ctx.font="13px system-ui";const tw=ctx.measureText(label).width+24;const x=Math.max(8,Math.min(W-tw-8,hovered.x-tw/2)),y=Math.max(22,hovered.y-25);ctx.fillStyle="#173629";ctx.fillRect(x,y-23,tw,28);ctx.strokeStyle="#4da779";ctx.strokeRect(x,y-23,tw,28);ctx.fillStyle="#eafff3";ctx.fillText(label,x+12,y-5)}}
 if(!drag&&!reduced&&!document.hidden)yaw+=.00055*Math.min(50,Math.max(0,t-last||16));
 last=t;requestAnimationFrame(draw);
}
function center(feature){
 const rings=feature.geometry.type==="Polygon"?feature.geometry.coordinates:feature.geometry.type==="MultiPolygon"?feature.geometry.coordinates.flat():[];
 let best=null,largest=0;
 for(const ring of rings){const pts=ring[0]&&typeof ring[0][0]==="number"?ring:ring[0];if(!pts||!pts.length)continue;
 let minX=180,maxX=-180,minY=90,maxY=-90;
 for(const p of pts){minX=Math.min(minX,p[0]);maxX=Math.max(maxX,p[0]);minY=Math.min(minY,p[1]);maxY=Math.max(maxY,p[1])}
 const area=(maxX-minX)*(maxY-minY);if(area>largest){largest=area;best={lon:(maxX+minX)/2,lat:(maxY+minY)/2}}}
 return best;
}
const centers=new Map();
async function loadMap(){
 try{
 const response=await fetch("https://raw.githubusercontent.com/johan/world.geo.json/master/countries.geo.json",{cache:"force-cache"});
 if(!response.ok)throw Error("Map data unavailable");
 const geo=await response.json();
 for(const f of geo.features||[]){
 const geoRings=f.geometry?.type==="Polygon"?f.geometry.coordinates:f.geometry?.type==="MultiPolygon"?f.geometry.coordinates.flat():[];
 for(const ring of geoRings){if(ring?.length)world.push(ring)}
 const c=center(f);if(c)centers.set(norm(f.properties?.name),c);
 }
 }catch(e){err.textContent="Country outlines unavailable. Live country counts can still display."}
}
async function refresh(){
 try{
 const [live,st]=await Promise.all([
 fetch("/api/public/radio",{cache:"no-store"}).then(r=>{if(!r.ok)throw Error("Radio API unavailable");return r.json()}),
 fetch("/statistics",{cache:"no-store"}).then(r=>r.ok?r.json():null)
 ]);
 document.getElementById("n-listeners").textContent=live.total_listeners||0;
 document.getElementById("n-playlists").textContent=0;
 const merged=new Map();
 for(const station of Object.values(st?.stations||{})){
 for(const c of station.active_listener_countries||[]){
 if(!c.country||!c.sessions)continue;
 const code=norm(c.country);const entry=merged.get(code)||{name:c.country,count:0};
 entry.count+=c.sessions;merged.set(code,entry);
 }
 }
 countries=[...merged.values()].sort((a,b)=>b.count-a.count).map(c=>{
 const name=aliases[c.name]||c.name;
 const coords=centers.get(norm(name))||centers.get(norm(c.name));
 return {...c,...(coords||{})};
 });
 document.getElementById("n-countries").textContent=countries.length;
 list.replaceChildren();
 for(const c of countries.slice(0,12)){
 const row=document.createElement("div");row.className="geo-row";
 const left=document.createElement("div"),name=document.createElement("div"),bar=document.createElement("div"),fill=document.createElement("i"),count=document.createElement("span");
 name.className="geo-name";name.textContent=c.name;bar.className="geo-bar";fill.style.width=(100*c.count/countries[0].count)+"%";bar.append(fill);left.append(name,bar);count.className="geo-number";count.textContent=c.count;row.append(left,count);list.append(row);
 }
 if(!countries.length){list.textContent=st?"No listeners with country information currently connected.":"Enable [stats] db_path to see live country totals."}
 }catch(e){err.textContent=e.message}
}
function pointer(e){const r=canvas.getBoundingClientRect();return{x:(e.clientX-r.left)*W/r.width,y:(e.clientY-r.top)*H/r.height}}
canvas.addEventListener("pointerdown",e=>{drag=true;moved=false;downX=e.clientX;downY=e.clientY;canvas.setPointerCapture(e.pointerId)});
canvas.addEventListener("pointermove",e=>{
 if(drag){const dx=e.clientX-downX,dy=e.clientY-downY;downX=e.clientX;downY=e.clientY;if(Math.abs(dx)+Math.abs(dy)>1)moved=true;yaw+=dx*.007;pitch=Math.max(-1.25,Math.min(1.25,pitch+dy*.004));return}
 const p=pointer(e);tooltip=null;for(const c of countries){const q=position(c.lon,c.lat);if(q.z>0&&Math.hypot(p.x-q.x,p.y-q.y)<12){tooltip=c;break}}canvas.title=tooltip?tooltip.name+": "+tooltip.count+" listeners":"Drag to rotate";
});
canvas.addEventListener("pointerup",()=>{drag=false});canvas.addEventListener("pointercancel",()=>{drag=false});canvas.addEventListener("pointerleave",()=>{tooltip=null;drag=false});
if(document.visibilityState==="visible")requestAnimationFrame(draw);
else requestAnimationFrame(draw);
Promise.all([loadMap(),refresh()]).then(()=>{});
setInterval(refresh,15000);
})();`
