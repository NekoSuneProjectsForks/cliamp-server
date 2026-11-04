package server

import (
 "encoding/json"
 "net/http"
 "net/url"
 "strings"
 "sync/atomic"
 "time"

 "cliamp-server/config"
)

// GameRadio is intentionally public and privacy-safe: it contains no
// listener IP addresses, credentials, storage paths or internal URLs.
type GameRadio struct {
 ID string `json:"id"`
 Name string `json:"name"`
 Description string `json:"description"`
 Genre string `json:"genre"`
 Online bool `json:"online"`
 Listeners int `json:"listeners"`
 TrackCount int `json:"track_count"`
 StreamURL string `json:"stream_url"`
 VRChatHLSURL string `json:"vrchat_hls_url,omitempty"`
 PreferredURL string `json:"preferred_url"`
 Format string `json:"format"`
 ArtworkURL string `json:"artwork_url"`
 NowPlaying struct{
  Title string `json:"title"`
  Artist string `json:"artist"`
  Album string `json:"album"`
 } `json:"now_playing"`
}

func gameBaseURL(r *http.Request) string {
 scheme:="http"
 if r.TLS!=nil{scheme="https"}
 if v:=strings.ToLower(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")));v=="http"||v=="https"{scheme=v}
 // r.Host is vetted by net/http for invalid characters and passed unchanged.
 return scheme+"://"+r.Host
}

func registerGameRadioAPI(mux *http.ServeMux,cfg *config.Config,stations map[string]*Station,switches map[string]*atomic.Bool) {
 build:=func(r *http.Request,id string,st *Station) GameRadio {
  base:=gameBaseURL(r)
  audio:=base+"/"+url.PathEscape(id)+"/stream"
  value:=GameRadio{
   ID:id,Name:st.Config.Name,Description:st.Config.Description,Genre:st.Config.Genre,
   Online:switches[id].Load(),Listeners:st.Hub.ListenerCount(),
   TrackCount:stationTrackCount(st),StreamURL:audio,PreferredURL:audio,
   Format:"mp3",ArtworkURL:base+"/logo.svg",
  }
  if st.Config.VRChatHLS {
   value.VRChatHLSURL=base+"/"+url.PathEscape(id)+"/vrchat/index.m3u8"
   value.PreferredURL=value.VRChatHLSURL
   value.Format="hls"
  }
  if !value.Online{value.Listeners=0}
  track:=st.Hub.CurrentTrack()
  value.NowPlaying.Title=track.Title
  value.NowPlaying.Artist=track.Artist
  value.NowPlaying.Album=track.Album
  return value
 }
 mux.HandleFunc("GET /api/v1/radios",func(w http.ResponseWriter,r *http.Request){
  onlyOnline:=r.URL.Query().Get("online_only")=="true"
  result:=make([]GameRadio,0,len(stations))
  for _,id:=range cfg.StationOrder{
   st,ok:=stations[id]
   if !ok || (onlyOnline && !switches[id].Load()){continue}
   result=append(result,build(r,id,st))
  }
  w.Header().Set("Content-Type","application/json; charset=utf-8")
  w.Header().Set("Cache-Control","no-store")
  _=json.NewEncoder(w).Encode(struct{
   Version int `json:"version"`
   UpdatedAt string `json:"updated_at"`
   Count int `json:"count"`
   Radios []GameRadio `json:"radios"`
  }{Version:1,UpdatedAt:time.Now().UTC().Format(time.RFC3339),Count:len(result),Radios:result})
 })
 mux.HandleFunc("GET /api/v1/radios/{id}",func(w http.ResponseWriter,r *http.Request){
  id:=r.PathValue("id")
  st,ok:=stations[id]
  if !ok{http.NotFound(w,r);return}
  w.Header().Set("Content-Type","application/json; charset=utf-8")
  w.Header().Set("Cache-Control","no-store")
  _=json.NewEncoder(w).Encode(build(r,id,st))
 })
}
