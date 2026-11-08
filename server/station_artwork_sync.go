package server

import (
 "context"
 "errors"
 "io"
 "log/slog"
 "net/http"
 "net/url"
 "strings"
 "sync/atomic"
 "time"

 "cliamp-server/vrchathls"
)

// The radio artwork service owns all third-party artwork requests. API requests
// and the HLS frame feeder read only files on the local persistent /data volume.
// One worker per station polls track identity; it never fetches artwork per user.
func runStationArtworkSync(ctx context.Context,stations map[string]*Station,switches map[string]*atomic.Bool,covers *artworkLookup){
 for id,station:=range stations{
  if _,ok:=switches[id];!ok{continue}
  go syncOneStationArtwork(ctx,id,station,covers)
 }
}

func syncOneStationArtwork(ctx context.Context,id string,station *Station,covers *artworkLookup){
 if err:=vrchathls.SaveFallbackCover(id,cdJewelCasePNG());err!=nil{
  slog.Warn("cannot prepare station CD cover","station",id,"error",err)
 }
 // Keep HLS's art.png updated even if VRChat HLS is disabled.
 ticker:=time.NewTicker(5*time.Second)
 defer ticker.Stop()
 var lastSong string
 var nextRetry time.Time
 var lastSource string
 var lastStamp time.Time
 tick:=func(){
  track:=station.Hub.CurrentTrack()
  key:=strings.ToLower(strings.TrimSpace(track.Artist))+"\x00"+strings.ToLower(strings.TrimSpace(track.Title))
  if track.Title!="" && track.Artist!="" && (key!=lastSong||time.Now().After(nextRetry)){
   lastSong=key
   // Negative lookups are throttled. Tracks with already-saved art never
   // trigger network lookups, including after an application restart.
   nextRetry=time.Now().Add(30*time.Minute)
   if !vrchathls.HasTrackCover(id,track.Artist,track.Title){
    searchCtx,cancel:=context.WithTimeout(ctx,9*time.Second)
    endpoint:=covers.coverWithContext(searchCtx,track.Artist,track.Title,track.Album)
    if endpoint!=""{
     if bytes,err:=downloadVerifiedCover(searchCtx,endpoint);err==nil{
      if err:=vrchathls.SaveAutoTrackCover(id,track.Artist,track.Title,bytes);err!=nil{
       slog.Warn("could not store cached album artwork","station",id,"error",err)
      }
     }else{slog.Debug("cover fetch unavailable; using local fallback","station",id,"error",err)}
    }
    cancel()
   }
  }
  source,stamp,err:=vrchathls.UpdateCurrentArt(id,track.Artist,track.Title,lastSource,lastStamp)
  if err!=nil {
   slog.Warn("cannot update local station artwork","station",id,"error",err)
  }else{lastSource,lastStamp=source,stamp}
 }
 tick()
 for{
  select{
  case <-ctx.Done():return
  case <-ticker.C:tick()
  }
 }
}

// Network access occurs only in the background station-artwork worker.
// Strict image-host validation prevents the public API from being used as
// an HTTP proxy. Image bytes are normalized to PNG by SaveAutoTrackCover.
var (
 errInvalidArtworkHost=errors.New("artwork host not trusted")
 errArtworkHTTP=errors.New("artwork download unsuccessful")
 errArtworkType=errors.New("artwork response is not an image")
 errArtworkSize=errors.New("artwork size invalid")
)
func downloadVerifiedCover(ctx context.Context,rawURL string)([]byte,error){
 parsed,err:=url.Parse(rawURL)
 if err!=nil{return nil,err}
 hostname:=strings.ToLower(parsed.Hostname())
 if parsed.Scheme!="https" || !(hostname=="mzstatic.com"||strings.HasSuffix(hostname,".mzstatic.com")){
  return nil,errInvalidArtworkHost
 }
 req,err:=http.NewRequestWithContext(ctx,http.MethodGet,parsed.String(),nil)
 if err!=nil{return nil,err}
 client:=&http.Client{Timeout:6*time.Second,CheckRedirect:func(*http.Request,[]*http.Request)error{return http.ErrUseLastResponse}}
 res,err:=client.Do(req)
 if err!=nil{return nil,err}
 defer res.Body.Close()
 if res.StatusCode!=http.StatusOK{return nil,errArtworkHTTP}
 if !strings.HasPrefix(strings.ToLower(res.Header.Get("Content-Type")),"image/"){return nil,errArtworkType}
 data,err:=io.ReadAll(io.LimitReader(res.Body,4<<20+1))
 if err!=nil{return nil,err}
 if len(data)==0||len(data)>4<<20{return nil,errArtworkSize}
 return data,nil
}
