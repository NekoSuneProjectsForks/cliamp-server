package server

import (
 "context"
 "errors"
 "io"
 "log/slog"
 "net/http"
 "net/url"
 "strings"
 "sync"
 "sync/atomic"
 "time"

 "cliamp-server/library"
 "cliamp-server/vrchathls"
)

var artworkDownloadMu sync.Mutex

// Artwork lookups are serialized across all stations and admin rescans, so a
// Raspberry Pi never downloads dozens of covers in parallel.
func cacheTrackArtwork(ctx context.Context,id string,track library.Track,covers *artworkLookup,force bool)(string,error){
 if strings.TrimSpace(track.Title)==""||strings.TrimSpace(track.Artist)==""{return "missing",nil}
 identity:=library.ArtworkIdentity(id,track.Path,track.Artist,track.Title,track.Album)
 artworkDownloadMu.Lock()
 defer artworkDownloadMu.Unlock()
 cached,exists,err:=library.FindArtwork(identity)
 if err!=nil{return "failed",err}
 if !force && exists {
  // A DB record is authoritative until the admin requests a refresh.
  return cached.Status,nil
 }
 if !force && vrchathls.HasTrackCover(id,track.Artist,track.Title) {
  identity.Status="found"
  return "found",library.SaveArtwork(identity)
 }
 if !force && vrchathls.HasAutoTrackCover(id,track.Artist,track.Title){
  identity.Status="found"
  return "found",library.SaveArtwork(identity)
 }
 searchCtx,cancel:=context.WithTimeout(ctx,12*time.Second)
 defer cancel()
 found:=covers.coverWithContext(searchCtx,track.Artist,track.Title,track.Album)
 identity.CoverURL=found
 identity.Status="missing"
 if found!="" {
  cover,err:=downloadVerifiedCover(searchCtx,found)
  if err==nil {
   if err=vrchathls.SaveCachedTrackCover(id,track.Artist,track.Title,cover);err==nil{
    identity.Status="found"
   }
  }
  if err!=nil{
   identity.Status="failed"
   if saveErr:=library.SaveArtwork(identity);saveErr!=nil{return "failed",saveErr}
   return "failed",err
  }
 }
 return identity.Status,library.SaveArtwork(identity)
}

func runStationArtworkSync(ctx context.Context,stations map[string]*Station,switches map[string]*atomic.Bool,covers *artworkLookup){
 for id,st:=range stations{
  if switches[id]==nil{continue}
  go syncOneStationArtwork(ctx,id,st,covers)
 }
}

func syncOneStationArtwork(ctx context.Context,id string,station *Station,covers *artworkLookup){
 if err:=vrchathls.SaveFallbackCover(id,cdJewelCasePNG());err!=nil{
  slog.Warn("cannot prepare station CD cover","station",id,"error",err)
 }
 ticker:=time.NewTicker(5*time.Second)
 defer ticker.Stop()
 var lastSource string
 var lastStamp time.Time
 var lastKey string
 tick:=func(){
  current:=station.Hub.CurrentTrack()
  update:=func(){
   source,stamp,err:=vrchathls.UpdateCurrentArt(id,current.Artist,current.Title,lastSource,lastStamp)
   if err!=nil{slog.Warn("cannot update local artwork","station",id,"error",err)}else{lastSource,lastStamp=source,stamp}
  }
  update()
  key:=current.Path+"\\x00"+current.Artist+"\\x00"+current.Title
  if key!=lastKey&&current.Title!=""&&current.Artist!=""{
   lastKey=key
   track:=library.Track{Path:current.Path,Artist:current.Artist,Title:current.Title,Album:current.Album}
   state,err:=cacheTrackArtwork(ctx,id,track,covers,false)
   if err!=nil{slog.Warn("artwork database or provider error; CD fallback remains","station",id,"error",err)}
   if state=="found"{slog.Debug("using artwork match from database","station",id)}
  }
  latest:=station.Hub.CurrentTrack()
  if latest.Path==current.Path&&latest.Title==current.Title&&latest.Artist==current.Artist{update()}
 }
 tick()
 for{
  select{
  case <-ctx.Done():return
  case <-ticker.C:tick()
  }
 }
}

var (
 errInvalidArtworkHost=errors.New("artwork host not trusted")
 errArtworkHTTP=errors.New("artwork download unsuccessful")
 errArtworkType=errors.New("artwork response is not an image")
 errArtworkSize=errors.New("artwork size invalid")
)
func downloadVerifiedCover(ctx context.Context,rawURL string)([]byte,error){
 parsed,err:=url.Parse(rawURL);if err!=nil{return nil,err}
 hostname:=strings.ToLower(parsed.Hostname())
 if parsed.Scheme!="https" || !(hostname=="mzstatic.com"||strings.HasSuffix(hostname,".mzstatic.com")){
  return nil,errInvalidArtworkHost
 }
 req,err:=http.NewRequestWithContext(ctx,http.MethodGet,parsed.String(),nil);if err!=nil{return nil,err}
 client:=&http.Client{Timeout:6*time.Second,CheckRedirect:func(*http.Request,[]*http.Request)error{return http.ErrUseLastResponse}}
 res,err:=client.Do(req);if err!=nil{return nil,err}
 defer res.Body.Close()
 if res.StatusCode!=http.StatusOK{return nil,errArtworkHTTP}
 if !strings.HasPrefix(strings.ToLower(res.Header.Get("Content-Type")),"image/"){return nil,errArtworkType}
 data,err:=io.ReadAll(io.LimitReader(res.Body,4<<20+1))
 if err!=nil{return nil,err}
 if len(data)==0||len(data)>4<<20{return nil,errArtworkSize}
 return data,nil
}
