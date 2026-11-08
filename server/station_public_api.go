package server

import (
 "context"
 "encoding/json"
 "io"
 "net/http"
 "net/url"
 "os"
 "strings"
 "sync/atomic"
 "time"

 "cliamp-server/vrchathls"
)

// PublicStationRadio represents one station only. No listener IPs, admin
// credentials, internal paths, or private music-library URLs are exposed.
type PublicStationRadio struct {
 ID string `json:"id"`
 Name string `json:"name"`
 Description string `json:"description"`
 Genre string `json:"genre"`
 Listeners int `json:"listeners"`
 Tracks int `json:"tracks"`
 Online bool `json:"online"`
 CurrentTrack struct {
  Title string `json:"title"`
  Artist string `json:"artist"`
  Album string `json:"album"`
  CoverURL string `json:"cover_url"`
 } `json:"current_track"`
 Stream string `json:"stream"`
}

func stationRadioSnapshot(id string, st *Station, enabled bool, covers *artworkLookup) PublicStationRadio {
 result:=PublicStationRadio{
  ID:id,Name:st.Config.Name,Description:st.Config.Description,
  Genre:st.Config.Genre,Tracks:stationTrackCount(st),Online:enabled,
  Stream:"/"+id+"/stream",
 }
 if enabled {result.Listeners=st.Hub.ListenerCount()}
 track:=st.Hub.CurrentTrack()
 result.CurrentTrack.Title=track.Title
 result.CurrentTrack.Artist=track.Artist
 result.CurrentTrack.Album=track.Album
 result.CurrentTrack.CoverURL=covers.cover(track.Artist,track.Title,track.Album)
 return result
}

// The new station routes reuse the same public information as /api/public/radio.
// /nowplaying deliberately returns the full station snapshot the API consumer
// requested, not a global station list.
func registerStationPublicRadioAPI(mux *http.ServeMux, stations map[string]*Station, switches map[string]*atomic.Bool, covers *artworkLookup) {
 send:=func(w http.ResponseWriter,r *http.Request){
  id:=r.PathValue("id")
  st,ok:=stations[id]
  if !ok || switches[id]==nil{http.NotFound(w,r);return}
  w.Header().Set("Content-Type","application/json; charset=utf-8")
  w.Header().Set("Cache-Control","no-store")
  w.Header().Set("X-Content-Type-Options","nosniff")
  _=json.NewEncoder(w).Encode(stationRadioSnapshot(id,st,switches[id].Load(),covers))
 }
 mux.HandleFunc("GET /api/{id}",send)
 mux.HandleFunc("GET /api/{id}/{$}",send)
 mux.HandleFunc("GET /api/{id}/nowplaying",send)
 mux.HandleFunc("GET /api/{id}/art.png",func(w http.ResponseWriter,r *http.Request){
  id:=r.PathValue("id")
  st,ok:=stations[id]
  if !ok || switches[id]==nil{http.NotFound(w,r);return}
  // Prefer materialized current artwork when present; no new work is needed.
  if path,err:=vrchathls.CurrentArtPath(id);err==nil{
   if fileErr:=func()error{
    file,err:=os.Open(path)
    if err!=nil{return err}
    defer file.Close()
    stat,err:=file.Stat()
    if err!=nil{return err}
    if !stat.Mode().IsRegular() || stat.Size()>4<<20 || stat.Size()==0{return os.ErrNotExist}
    content,err:=io.ReadAll(io.LimitReader(file,4<<20+1))
    if err!=nil || len(content)>4<<20{return os.ErrNotExist}
    w.Header().Set("Cache-Control","no-store")
    w.Header().Set("Content-Type","image/png")
    w.Header().Set("X-Content-Type-Options","nosniff")
    _,_=w.Write(content)
    return nil
   }();fileErr==nil{return}
  }
  track:=st.Hub.CurrentTrack()
  endpoint:=covers.cover(track.Artist,track.Title,track.Album)
  parsed,err:=url.Parse(endpoint)
  if err!=nil || parsed.Scheme!="https" || !(parsed.Hostname()=="mzstatic.com"||strings.HasSuffix(parsed.Hostname(),".mzstatic.com")){
   http.NotFound(w,r);return
  }
  ctx,cancel:=context.WithTimeout(r.Context(),5*time.Second)
  defer cancel()
  request,err:=http.NewRequestWithContext(ctx,http.MethodGet,parsed.String(),nil)
  if err!=nil{http.NotFound(w,r);return}
  client:=&http.Client{Timeout:5*time.Second,CheckRedirect:func(*http.Request,[]*http.Request)error{return http.ErrUseLastResponse}}
  response,err:=client.Do(request)
  if err!=nil{http.Error(w,"Artwork temporarily unavailable",http.StatusBadGateway);return}
  defer response.Body.Close()
  if response.StatusCode!=http.StatusOK || !strings.HasPrefix(strings.ToLower(response.Header.Get("Content-Type")),"image/"){
   http.Error(w,"Artwork temporarily unavailable",http.StatusBadGateway);return
  }
  content,err:=io.ReadAll(io.LimitReader(response.Body,4<<20+1))
  if err!=nil || len(content)>4<<20{http.Error(w,"Artwork too large or unavailable",http.StatusBadGateway);return}
  w.Header().Set("Cache-Control","no-store")
  w.Header().Set("Content-Type",response.Header.Get("Content-Type"))
  w.Header().Set("X-Content-Type-Options","nosniff")
  w.WriteHeader(http.StatusOK)
  _,_=w.Write(content)
 })
}

