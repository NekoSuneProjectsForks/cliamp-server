package server

import (
 "encoding/json"
 "net/http"
 "net/url"
 "sync/atomic"

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

func stationRadioSnapshot(id string, st *Station, enabled bool, baseURL string) PublicStationRadio {
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
 result.CurrentTrack.CoverURL=baseURL+"/api/"+url.PathEscape(id)+"/art.png"
 return result
}

// The new station routes reuse the same public information as /api/public/radio.
// /nowplaying deliberately returns the full station snapshot the API consumer
// requested, not a global station list.
func registerStationPublicRadioAPI(mux *http.ServeMux, stations map[string]*Station, switches map[string]*atomic.Bool) {
 send:=func(w http.ResponseWriter,r *http.Request){
  id:=r.PathValue("id")
  st,ok:=stations[id]
  if !ok || switches[id]==nil{http.NotFound(w,r);return}
  w.Header().Set("Content-Type","application/json; charset=utf-8")
  w.Header().Set("Cache-Control","no-store")
  w.Header().Set("X-Content-Type-Options","nosniff")
  _=json.NewEncoder(w).Encode(stationRadioSnapshot(id,st,switches[id].Load(),gameBaseURL(r)))
 }
 mux.HandleFunc("GET /api/{id}",send)
 mux.HandleFunc("GET /api/{id}/{$}",send)
 mux.HandleFunc("GET /api/{id}/nowplaying",send)
 mux.HandleFunc("GET /api/{id}/art.png",func(w http.ResponseWriter,r *http.Request){
  id:=r.PathValue("id")
  _,ok:=stations[id]
  if !ok || switches[id]==nil{http.NotFound(w,r);return}
  serveStationLocalArt(w,id)
 })
}


// serveStationLocalArt is shared by /api/{station}/art.png and the original
// /{station}/art.png URL; neither path ever fetches a remote image.
func serveStationLocalArt(w http.ResponseWriter,id string){
 if data,err:=vrchathls.ReadCurrentArt(id);err==nil{
  w.Header().Set("Cache-Control","no-store")
  w.Header().Set("Content-Type","image/png")
  w.Header().Set("X-Content-Type-Options","nosniff")
  _,_=w.Write(data)
  return
 }
 serveCDJewelCase(w)
}
