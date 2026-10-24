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
 "cliamp-server/library"
 "cliamp-server/s3store"
 "context"
 "fmt"
 "sync"
)

var libraryEditMu sync.Mutex

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
 mux.HandleFunc("GET /assets/radio-globe.js",func(w http.ResponseWriter,r *http.Request){w.Header().Set("Content-Type","application/javascript; charset=utf-8");w.Header().Set("Cache-Control","public, max-age=300");_,_=w.Write([]byte(radioGlobeJS))})
 mux.HandleFunc("GET /docs",func(w http.ResponseWriter,r *http.Request){w.Header().Set("Content-Type","text/html; charset=utf-8");w.Header().Set("Cache-Control","public, max-age=300");_,_=w.Write([]byte(publicDocsHTML))})
 mux.HandleFunc("GET /admin",func(w http.ResponseWriter,r *http.Request){w.Header().Set("Content-Type","text/html; charset=utf-8");w.Header().Set("Cache-Control","no-store");_,_=w.Write([]byte(adminHTML))})
 artwork:=newArtworkLookup()
 mux.HandleFunc("GET /api/public/radio",func(w http.ResponseWriter,r *http.Request) {
  type song struct { Title string `json:"title"`; Artist string `json:"artist"`; Album string `json:"album"`; CoverURL string `json:"cover_url"` }
  type item struct { ID string `json:"id"`; Name string `json:"name"`; Description string `json:"description"`; Genre string `json:"genre"`; Listeners int `json:"listeners"`; Tracks int `json:"tracks"`; Online bool `json:"online"`; CurrentTrack song `json:"current_track"`; Stream string `json:"stream"` }
  out := struct { Stations []item `json:"stations"`; TotalListeners int `json:"total_listeners"` }{Stations:make([]item,0,len(stations))}
  for _,id:=range cfg.StationOrder {
   st,ok:=stations[id];if !ok {continue}
   t:=st.Hub.CurrentTrack(); enabled:=switches[id].Load()
   listeners:=st.Hub.ListenerCount(); if !enabled {listeners=0}
   out.Stations=append(out.Stations,item{ID:id,Name:st.Config.Name,Description:st.Config.Description,Genre:st.Config.Genre,Listeners:listeners,Tracks:stationTrackCount(st),Online:enabled,CurrentTrack:song{t.Title,t.Artist,t.Album,artwork.cover(t.Artist,t.Title,t.Album)},Stream:"/"+id+"/stream"})
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
  if err:=syncStationLibrary(st);err!=nil{http.Error(w,"Uploaded but failed to refresh library: "+err.Error(),500);return}
  _=json.NewEncoder(w).Encode(map[string]any{"uploaded":filename,"bytes":n,"message":"Uploaded and queued for rotation without interrupting listeners."})
 })
 mux.HandleFunc("GET /api/admin/stations/{id}/library",func(w http.ResponseWriter,r *http.Request){
  if !admin(w,r){return}
  st,ok:=stations[r.PathValue("id")];if !ok{http.NotFound(w,r);return}
  tracks,err:=stationLibrary(r.Context(),st,r.PathValue("id"))
  if err!=nil{http.Error(w,"Cannot scan library",500);return}
  type row struct{Filename string `json:"filename"`;Title string `json:"title"`;Artist string `json:"artist"`;Album string `json:"album"`}
  result:=make([]row,0,len(tracks))
  for _,t:=range tracks{filename:=t.Path; if _,key,ok:=s3store.DecodePath(t.Path);ok{filename=key}else{var err error;filename,err=filepath.Rel(st.Config.Path,t.Path);if err!=nil||strings.HasPrefix(filename,".."){continue}};result=append(result,row{filename,t.Title,t.Artist,t.Album})}
  w.Header().Set("Content-Type","application/json");_ = json.NewEncoder(w).Encode(map[string]any{"station":r.PathValue("id"),"tracks":result})
 })
 mux.HandleFunc("DELETE /api/admin/stations/{id}/library",func(w http.ResponseWriter,r *http.Request){
  if !admin(w,r){return}
  st,ok:=stations[r.PathValue("id")];if !ok{http.NotFound(w,r);return}
  var body struct{ Filename string `json:"filename"`}
  if json.NewDecoder(http.MaxBytesReader(w,r.Body,2048)).Decode(&body)!=nil{http.Error(w,"Invalid JSON",400);return}
  filename:=filepath.Clean(body.Filename)
  if filename=="."||filename==""||filepath.IsAbs(filename)||strings.HasPrefix(filename,"..")||strings.Contains(filename,":"){http.Error(w,"Invalid filename",400);return}
  target:=filepath.Join(st.Config.Path,filename)
  info,err:=os.Lstat(target);if err!=nil||!info.Mode().IsRegular(){http.Error(w,"File not found",404);return}
  libraryEditMu.Lock();defer libraryEditMu.Unlock()
  tracks,err:=library.Scan(st.Config.Path,st.Config.Recursive);if err!=nil{http.Error(w,"Cannot scan library",500);return}
  if len(tracks)<=1{http.Error(w,"Cannot delete the final music track",409);return}
  if err:=os.Remove(target);err!=nil{http.Error(w,"Cannot delete music file",500);return}
  if err:=syncStationLibrary(st);err!=nil{http.Error(w,"Deleted file but failed to update rotation",500);return}
  w.Header().Set("Content-Type","application/json");_ = json.NewEncoder(w).Encode(map[string]any{"deleted":filename,"message":"Track removed from future rotation; stream remains connected."})
 })

 // Ads and jingles are separate per-station media libraries.
 mediaPath:=func(st *Station,kind string) string {
  switch kind {case "ads":return st.Config.AdsPath;case "jingles":return st.Config.JinglesPath}
  return ""
 }
 refreshMedia:=func(st *Station) error {
  if st.Scheduler==nil{return nil}
  return st.Scheduler.ReloadPools(st.Config.AdsPath,st.Config.AdShuffle,st.Config.JinglesPath,st.Config.JingleShuffle)
 }
 mux.HandleFunc("GET /api/admin/stations/{id}/media/{kind}",func(w http.ResponseWriter,r *http.Request){
  if !admin(w,r){return}
  st,ok:=stations[r.PathValue("id")];if !ok{http.NotFound(w,r);return}
  path:=mediaPath(st,r.PathValue("kind"))
  if path==""{http.Error(w,"Configure the media directory in config.toml first",409);return}
  info,err:=os.Stat(path);if err!=nil||!info.IsDir(){http.Error(w,"Media path must be an existing directory",409);return}
  entries,err:=os.ReadDir(path);if err!=nil{http.Error(w,"Cannot read media directory",500);return}
  files:=make([]string,0)
  for _,f:=range entries{if f.Type().IsRegular()&&uploadName.MatchString(f.Name()){files=append(files,f.Name())}}
  w.Header().Set("Content-Type","application/json");_ = json.NewEncoder(w).Encode(map[string]any{"files":files,"kind":r.PathValue("kind")})
 })
 mux.HandleFunc("POST /api/admin/stations/{id}/media/{kind}",func(w http.ResponseWriter,r *http.Request){
  if !admin(w,r){return}
  st,ok:=stations[r.PathValue("id")];if !ok{http.NotFound(w,r);return}
  path:=mediaPath(st,r.PathValue("kind"))
  if path==""{http.Error(w,"Media directory not configured",409);return}
  info,err:=os.Stat(path);if err!=nil||!info.IsDir(){http.Error(w,"Media path must be an existing directory",409);return}
  r.Body=http.MaxBytesReader(w,r.Body,256<<20)
  if err:=r.ParseMultipartForm(8<<20);err!=nil{http.Error(w,"Invalid or oversized upload",400);return}
  file,header,err:=r.FormFile("file");if err!=nil{http.Error(w,"Missing file",400);return};defer file.Close()
  name:=filepath.Base(header.Filename)
  if name!=header.Filename||!uploadName.MatchString(name){http.Error(w,"Invalid filename or audio type",400);return}
  libraryEditMu.Lock();defer libraryEditMu.Unlock()
  output,err:=os.OpenFile(filepath.Join(path,name),os.O_CREATE|os.O_EXCL|os.O_WRONLY,0644)
  if err!=nil{http.Error(w,"Cannot create media file",409);return}
  n,err:=io.Copy(output,io.LimitReader(file,256<<20+1));closeErr:=output.Close()
  if err!=nil||closeErr!=nil||n>256<<20{_ = os.Remove(filepath.Join(path,name));http.Error(w,"Upload failed",400);return}
  if err:=refreshMedia(st);err!=nil{http.Error(w,"Uploaded, but could not reload scheduler",500);return}
  w.Header().Set("Content-Type","application/json");_ = json.NewEncoder(w).Encode(map[string]any{"uploaded":name,"message":"Media uploaded; changes will apply at an upcoming song boundary."})
 })
 mux.HandleFunc("DELETE /api/admin/stations/{id}/media/{kind}",func(w http.ResponseWriter,r *http.Request){
  if !admin(w,r){return}
  st,ok:=stations[r.PathValue("id")];if !ok{http.NotFound(w,r);return}
  path:=mediaPath(st,r.PathValue("kind"))
  if path==""{http.Error(w,"Media directory not configured",409);return}
  var payload struct{Filename string `json:"filename"`}
  if json.NewDecoder(http.MaxBytesReader(w,r.Body,2048)).Decode(&payload)!=nil||payload.Filename!=filepath.Base(payload.Filename)||!uploadName.MatchString(payload.Filename){http.Error(w,"Invalid filename",400);return}
  target:=filepath.Join(path,payload.Filename)
  libraryEditMu.Lock();defer libraryEditMu.Unlock()
  info,err:=os.Lstat(target);if err!=nil||!info.Mode().IsRegular(){http.Error(w,"Media not found",404);return}
  if err:=os.Remove(target);err!=nil{http.Error(w,"Cannot delete media",500);return}
  if err:=refreshMedia(st);err!=nil{http.Error(w,"Deleted, but could not reload scheduler",500);return}
  w.Header().Set("Content-Type","application/json");_ = json.NewEncoder(w).Encode(map[string]any{"deleted":payload.Filename})
 })

}

func stationLibrary(ctx context.Context,st *Station,id string)([]library.Track,error){
 if st.Config.S3.Endpoint=="" {return library.Scan(st.Config.Path,st.Config.Recursive)}
 client,ok:=s3store.ClientFor(id);if !ok{return nil,fmt.Errorf("S3 station not configured")}
 objects,err:=client.List(ctx);if err!=nil{return nil,err}
 tracks:=make([]library.Track,0,len(objects))
 for _,o:=range objects{base:=filepath.Base(o.Key);tracks=append(tracks,library.Track{Path:s3store.TrackPath(id,o.Key),Title:strings.TrimSuffix(base,filepath.Ext(base))})}
 return tracks,nil
}
func stationTrackCount(st *Station)int{if st.Playlist!=nil{return st.Playlist.Len()};return st.TrackCount}
func syncStationLibrary(st *Station)error{
 if st.Playlist==nil{return nil}
 tracks,err:=library.Scan(st.Config.Path,st.Config.Recursive)
 if err!=nil{return err}
 if st.Config.IntroFile!="" {
  filtered:=tracks[:0];for _,t:=range tracks{if t.Path!=st.Config.IntroFile{filtered=append(filtered,t)}}
  tracks=filtered
 }
 return st.Playlist.Replace(tracks)

}
