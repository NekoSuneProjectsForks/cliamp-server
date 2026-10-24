package server

import (
 "encoding/json"
 "io"
 "net/http"
 "os"
 "path/filepath"
 "regexp"
 "strings"
 "sync/atomic"
 "time"
 "cliamp-server/broadcast"

 "cliamp-server/config"
)

var uploadName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9 _().,'-]{0,180}[.](?i:mp3|flac|wav|ogg|opus|aac|m4a|webm|wma)$`)

// registerDashboard adds a public privacy-preserving radio directory and
// authenticated per-station controls. No Docker socket or shell access needed.
func registerDashboard(mux *http.ServeMux, cfg *config.Config, stations map[string]*Station, switches map[string]*atomic.Bool) {
 mux.HandleFunc("GET /{$}", func(w http.ResponseWriter,r *http.Request) {
  if r.URL.Path != "/" && r.URL.Path != "/admin" { http.NotFound(w,r); return }
  w.Header().Set("Content-Type","text/html; charset=utf-8")
  w.Header().Set("Cache-Control","no-store")
  _,_ = w.Write([]byte(dashboardHTML))
 })
 mux.HandleFunc("GET /docs",func(w http.ResponseWriter,r *http.Request){w.Header().Set("Content-Type","text/html; charset=utf-8");w.Header().Set("Cache-Control","public, max-age=300");_,_=w.Write([]byte(publicDocsHTML))})
 mux.HandleFunc("GET /admin",func(w http.ResponseWriter,r *http.Request){w.Header().Set("Content-Type","text/html; charset=utf-8");w.Header().Set("Cache-Control","no-store");_,_=w.Write([]byte(dashboardHTML))})
 mux.HandleFunc("GET /api/public/radio",func(w http.ResponseWriter,r *http.Request) {
  type song struct { Title string `json:"title"`; Artist string `json:"artist"`; Album string `json:"album"` }
  type item struct { ID string `json:"id"`; Name string `json:"name"`; Description string `json:"description"`; Genre string `json:"genre"`; Listeners int `json:"listeners"`; Tracks int `json:"tracks"`; Online bool `json:"online"`; CurrentTrack song `json:"current_track"`; Stream string `json:"stream"` }
  out := struct { Stations []item `json:"stations"`; TotalListeners int `json:"total_listeners"` }{Stations:make([]item,0,len(stations))}
  for _,id:=range cfg.StationOrder {
   st,ok:=stations[id];if !ok {continue}
   t:=st.Hub.CurrentTrack(); enabled:=switches[id].Load()
   listeners:=st.Hub.ListenerCount(); if !enabled {listeners=0}
   out.Stations=append(out.Stations,item{ID:id,Name:st.Config.Name,Description:st.Config.Description,Genre:st.Config.Genre,Listeners:listeners,Tracks:st.TrackCount,Online:enabled,CurrentTrack:song{t.Title,t.Artist,t.Album},Stream:"/"+id+"/stream"})
   out.TotalListeners+=listeners
  }
  w.Header().Set("Cache-Control","no-store")
  w.Header().Set("Content-Type","application/json")
  _=json.NewEncoder(w).Encode(out)
 })
 // Mutations require a configured secret, constant-time verification by the
 // standard-library BasicAuth method is not guaranteed; compare via subtle.
 admin := func(w http.ResponseWriter,r *http.Request) bool {
  if cfg.Admin.Password=="" {http.Error(w,"Configure [admin] password first",503);return false}
  auth:=strings.TrimPrefix(r.Header.Get("Authorization"),"Bearer ")
  if !strings.HasPrefix(r.Header.Get("Authorization"),"Bearer ") || !secureCompare(auth,cfg.Admin.Password) {
   http.Error(w,"Unauthorized",401);return false
  }
  if r.Method!="GET" && r.Header.Get("Origin")!="" {
   // Only same-origin web UI may submit browser mutations.
   origin:=r.Header.Get("Origin")
   if origin!="http://"+r.Host && origin!="https://"+r.Host {
    http.Error(w,"Forbidden origin",403);return false
   }
  }
  return true
 }
 mux.HandleFunc("GET /api/admin/stations",func(w http.ResponseWriter,r *http.Request) {
  if !admin(w,r){return}
  w.Header().Set("Content-Type","application/json")
  type station struct { ID string `json:"id"`; Name string `json:"name"`; Path string `json:"path"`; Online bool `json:"online"`; Listeners int `json:"listeners"` }
  list:=make([]station,0,len(stations))
  for _,id:=range cfg.StationOrder{if st,ok:=stations[id];ok{list=append(list,station{id,st.Config.Name,st.Config.Path,switches[id].Load(),st.Hub.ListenerCount()})}}
  _=json.NewEncoder(w).Encode(list)
 })
 mux.HandleFunc("GET /api/admin/listeners",func(w http.ResponseWriter,r *http.Request){
  if !admin(w,r){return}
  type listener struct { IP string `json:"ip"`; Country string `json:"country"`; City string `json:"city"`; ConnectedAt string `json:"connected_at"`; DurationSeconds int64 `json:"duration_seconds"` }
  result:=map[string][]listener{}
  now:=time.Now()
  for id,st:=range stations {
   values:=[]listener{}
   for _,snap:=range st.Hub.Listeners(){values=append(values,listener{snap.Info.IP,snap.Info.Country,snap.Info.City,snap.ConnectedAt.UTC().Format(time.RFC3339),int64(now.Sub(snap.ConnectedAt).Seconds())})}
   result[id]=values
  }
  w.Header().Set("Cache-Control","no-store")
  w.Header().Set("Content-Type","application/json")
  _=json.NewEncoder(w).Encode(map[string]any{"stations":result})
 })
 mux.HandleFunc("GET /api/admin/bans",func(w http.ResponseWriter,r *http.Request){
  if !admin(w,r){return}
  w.Header().Set("Cache-Control","no-store");w.Header().Set("Content-Type","application/json")
  _=json.NewEncoder(w).Encode(map[string]any{"banned_ips":broadcast.ListBans()})
 })
 mux.HandleFunc("POST /api/admin/bans",func(w http.ResponseWriter,r *http.Request){
  if !admin(w,r){return}
  var payload struct { IP string `json:"ip"`; Banned *bool `json:"banned"` }
  if json.NewDecoder(http.MaxBytesReader(w,r.Body,2048)).Decode(&payload)!=nil || payload.Banned==nil {http.Error(w,"Invalid request",400);return}
  if err:=broadcast.SetBan(payload.IP,*payload.Banned);err!=nil{http.Error(w,err.Error(),400);return}
  w.Header().Set("Content-Type","application/json")
  _=json.NewEncoder(w).Encode(map[string]any{"ip":payload.IP,"banned":*payload.Banned})
 })
 mux.HandleFunc("POST /api/admin/stations/{id}/state",func(w http.ResponseWriter,r *http.Request) {
  if !admin(w,r){return}
  id:=r.PathValue("id");state,ok:=switches[id];if !ok{http.NotFound(w,r);return}
  var payload struct{Online *bool `json:"online"`}
  if json.NewDecoder(http.MaxBytesReader(w,r.Body,1024)).Decode(&payload)!=nil||payload.Online==nil {http.Error(w,"Invalid state",400);return}
  state.Store(*payload.Online)
  w.Header().Set("Content-Type","application/json")
  _=json.NewEncoder(w).Encode(map[string]any{"id":id,"online":state.Load()})
 })
 mux.HandleFunc("POST /api/admin/stations/{id}/upload",func(w http.ResponseWriter,r *http.Request) {
  if !admin(w,r){return}
  st,ok:=stations[r.PathValue("id")];if !ok{http.NotFound(w,r);return}
  r.Body=http.MaxBytesReader(w,r.Body,256<<20)
  if err:=r.ParseMultipartForm(8<<20);err!=nil{http.Error(w,"Invalid upload or exceeds 256 MiB",400);return}
  file,header,err:=r.FormFile("file");if err!=nil{http.Error(w,"Missing file",400);return};defer file.Close()
  filename:=filepath.Base(header.Filename)
  if !uploadName.MatchString(filename)||filename!=header.Filename {http.Error(w,"Unsupported file name/type",400);return}
  target:=filepath.Join(st.Config.Path,filename)
  // Exclusive create prevents accidental overwriting existing tracks.
  out,err:=os.OpenFile(target,os.O_WRONLY|os.O_CREATE|os.O_EXCL,0644)
  if err!=nil{http.Error(w,"Cannot create music file (check volume write access)",409);return}
  n,err:=io.Copy(out,io.LimitReader(file,256<<20+1))
  closeErr:=out.Close()
  if err!=nil||closeErr!=nil||n>256<<20 { _=os.Remove(target);http.Error(w,"Upload failed",400);return }
  w.Header().Set("Content-Type","application/json")
  _=json.NewEncoder(w).Encode(map[string]any{"uploaded":filename,"bytes":n,"message":"Uploaded. Restart cliamp-server to add this track to rotation."})
 })
}
