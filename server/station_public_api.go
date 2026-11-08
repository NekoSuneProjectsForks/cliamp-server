package server

import (
 "context"
 "encoding/json"
 "io"
 "net/http"
 "net/url"
 "strings"
 "sync/atomic"
 "time"

 "cliamp-server/config"
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
 lookup:=func(w http.ResponseWriter,id string)(*Station,bool){
  st,ok:=stations[id]
  if !ok || switches[id]==nil {
   http.NotFound(w,nil)
   return nil,false
  }
  return st,true
 }
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
  if _,err:=vrchathls.CurrentArtPath(id);err==nil{
   // Current art is available only if HLS visuals generated it.
   // ServeCurrentArt returns 404 when its file has not been created.
   // Use a recorder here so 404 can fall through to a verified iTunes cover.
   rec:=newArtworkResponseCapture()
   vrchathls.ServeCurrentArt(rec,r,id)
   if rec.code==http.StatusOK{
    w.Header().Set("Cache-Control","no-store")
    w.Header().Set("Content-Type","image/png")
    w.Header().Set("X-Content-Type-Options","nosniff")
    w.WriteHeader(http.StatusOK)
    _,_=w.Write(rec.data)
    return
   }
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
 _=lookup
}

// A small response writer lets us check whether a current-art file exists
// without leaking a 404 response before attempting the public fallback.
type artworkResponseCapture struct { header http.Header; code int; data []byte }
func newArtworkResponseCapture()*artworkResponseCapture{return &artworkResponseCapture{header:make(http.Header)}}
func (r *artworkResponseCapture)Header()http.Header{return r.header}
func (r *artworkResponseCapture)WriteHeader(status int){r.code=status}
func (r *artworkResponseCapture)Write(b []byte)(int,error){
 if r.code==0{r.code=http.StatusOK}
 r.data=append(r.data,b...)
 return len(b),nil
}
var _ = config.StationConfig{}
