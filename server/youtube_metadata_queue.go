package server

import (
 "context"
 "fmt"
 "log/slog"
 "sync"
 "strings"
 "errors"
 "time"

 "cliamp-server/library"
 "cliamp-server/config"
 "cliamp-server/youtubemeta"
)

// youtubeMetadataQueue runs one extractor at a time so browser requests do
// not wait for YouTube and a Raspberry Pi cannot spawn unlimited yt-dlp jobs.
type youtubeMetadataQueue struct {
 mu sync.Mutex
 jobs map[string]youtubeMetadataStatus
 pending chan youtubeMetadataTask
 stations map[string]*Station
 lookup func(context.Context,string)(youtubemeta.Metadata,error)
 artwork func(context.Context,string,string,string)(string,error)
 health *videoHealthStore
}
type youtubeMetadataStatus struct {
 State string `json:"state"`
 Error string `json:"error,omitempty"`
 Availability string `json:"availability,omitempty"`
 DeletedConfirmations int `json:"deleted_confirmations,omitempty"`
 Artist string `json:"artist,omitempty"`
 Title string `json:"title,omitempty"`
 UpdatedAt time.Time `json:"updated_at"`
}
type youtubeMetadataTask struct {
 id,url string
 force bool
}
func newYouTubeMetadataQueue(stations map[string]*Station, customLookup ...func(context.Context,string)(youtubemeta.Metadata,error))*youtubeMetadataQueue {
 q:=&youtubeMetadataQueue{
  jobs:make(map[string]youtubeMetadataStatus),pending:make(chan youtubeMetadataTask,512),stations:stations,lookup:youtubemeta.Lookup,artwork:cacheYouTubeArtwork,health:newVideoHealthStore(videoHealthFile()),
 }
 if len(customLookup)>0&&customLookup[0]!=nil{
  q.lookup=customLookup[0]
  // Unit tests inject metadata lookup and never make external cover calls.
  q.artwork=func(context.Context,string,string,string)(string,error){return "",nil}
 }
 go q.run()
 go q.recheckKnownUnavailable()
 return q
}
func metadataKey(id,url string)string{return id+"\x00"+url}
func (q *youtubeMetadataQueue) status(id,url string)(youtubeMetadataStatus,bool){
 q.mu.Lock();defer q.mu.Unlock()
 result,ok:=q.jobs[metadataKey(id,url)]
 return result,ok
}
func(q *youtubeMetadataQueue) set(id,url string,value youtubeMetadataStatus){
 q.mu.Lock();defer q.mu.Unlock()
 q.jobs[metadataKey(id,url)]=value
}
// enqueue returns instantly; the worker is intentionally rate limited to one
// yt-dlp process. Duplicate in-flight tasks are coalesced.
func(q *youtubeMetadataQueue) enqueue(id,url string,force bool)(youtubeMetadataStatus,error){
 if !isStationVideo(q.stations[id],url){return youtubeMetadataStatus{},fmt.Errorf("video not in station")}
 key:=metadataKey(id,url)
 q.mu.Lock()
 if prev,ok:=q.jobs[key];ok&&(prev.State=="queued"||prev.State=="running"){
  q.mu.Unlock();return prev,nil
 }
 now:=time.Now().UTC()
 status:=youtubeMetadataStatus{State:"queued",UpdatedAt:now}
 select{
 case q.pending<-youtubeMetadataTask{id:id,url:url,force:force}:
  q.jobs[key]=status
  q.mu.Unlock()
  return status,nil
 default:
  q.mu.Unlock()
  return youtubeMetadataStatus{},fmt.Errorf("metadata queue full; retry later")
 }
}
func isStationVideo(st *Station,url string)bool{
 if st==nil||st.Config.Source!="youtube"{return false}
 for _,s:=range st.Config.YouTubeURLs{if s==url{return true}}
 return false
}
func(q *youtubeMetadataQueue) run(){
 for task:=range q.pending{
  q.set(task.id,task.url,youtubeMetadataStatus{State:"running",UpdatedAt:time.Now().UTC()})
  // Deliberately independent of the originating HTTP request context.
  // Lookups have a strict 15s timeout inside youtubemeta.Lookup.
  ctx,cancel:=context.WithTimeout(context.Background(),18*time.Second)
  prior:=q.health.get(task.id,task.url)
  cached,cacheHit,cacheErr:=library.CachedYouTube(task.url)
  if cacheErr!=nil{slog.Warn("cannot load video ID metadata cache","error",cacheErr)}
  // Private/deleted entries must be revalidated against YouTube; cached
  // public metadata is not evidence that an unavailable video recovered.
  useCache:=cacheErr==nil&&cacheHit&&!task.force&&prior.State!="private"&&prior.State!="deleted"
  var meta youtubemeta.Metadata
  var err error
  if useCache{
   meta=youtubemeta.Metadata{Title:cached.Title,Artist:cached.Artist,Album:cached.Album,RawTitle:cached.OriginalTitle,Availability:prior.State}
   if meta.Availability==""{meta.Availability="public"}
  }else{
   meta,err=q.lookup(ctx,task.url)
  }
  cancel()
  if err==nil {
   // Persist new title metadata once; a cache hit never invokes yt-dlp.
   entry:=cached
   if !useCache {
    entry=library.SourceCacheEntry{Title:meta.Title,Artist:meta.Artist,Album:meta.Album,OriginalTitle:meta.RawTitle}
    if cacheHit{entry.ArtworkChecked=cached.ArtworkChecked;entry.ArtworkURL=cached.ArtworkURL}
   }
   // Artwork-only backfills are allowed from cached artist/title data.
   // This is at most one iTunes lookup per canonical video ID.
   needsSave:=!useCache
   if !entry.ArtworkChecked {
    artCtx,stop:=context.WithTimeout(context.Background(),8*time.Second)
    coverURL,coverErr:=q.artwork(artCtx,task.url,meta.Artist,meta.Title)
    stop()
    if coverErr!=nil{slog.Debug("no matching iTunes cover","video_id",library.YouTubeVideoID(task.url),"error",coverErr)}
    entry.ArtworkChecked=true
    entry.ArtworkURL=coverURL
    needsSave=true
   }
   if needsSave{
    if saveErr:=library.SaveCachedYouTube(task.url,entry);saveErr!=nil{
     slog.Warn("could not persist shared YouTube cache","error",saveErr)
    }
   }
   // Match the station's displayed metadata, which may contain manual edits.
   displayArtist,displayTitle:=meta.Artist,meta.Title
   if manual,exists,readErr:=library.MetadataFor(task.id,"youtube:"+task.url);readErr==nil&&exists{
    displayArtist,displayTitle=manual.Artist,manual.Title
   }
   reuseCachedCover(task.id,task.url,displayArtist,displayTitle)
   libraryEditMu.Lock()
   st:=q.stations[task.id]
   if !isStationVideo(st,task.url){
    err=fmt.Errorf("video removed while lookup was running")
   } else {
    path:="youtube:"+task.url
    _,exists,readErr:=library.MetadataFor(task.id,path)
    if readErr!=nil{err=readErr
    }else if !exists||task.force{
     err=library.SaveMetadata(task.id,path,library.Metadata{
      Title:meta.Title,Artist:meta.Artist,Album:meta.Album,SourceTitle:meta.RawTitle,
     })
     if err==nil{
      err=syncStationLibrary(st)
      if err==nil&&st.Hub!=nil{st.Hub.UpdateTrackMetadata(path,meta.Artist,meta.Title,meta.Album)}
     }
    }
   }
   libraryEditMu.Unlock()
  }
  result:=youtubeMetadataStatus{State:"completed",Artist:meta.Artist,Title:meta.Title,UpdatedAt:time.Now().UTC()}
  kind:=meta.Availability
  if kind==""{kind="public"}
  if err!=nil{
   kind=youtubemeta.ClassifyError(err)
   result.State="failed"
   result.Error="YouTube lookup unavailable; link retained. You can retry or edit metadata manually."
   if errors.Is(err,context.DeadlineExceeded){result.Error="YouTube metadata request timed out; link retained."}
   if strings.Contains(err.Error(),"invalid yt-dlp metadata field"){result.Error="yt-dlp returned an unsupported metadata value. Update/redeploy CLIAMP and retry."}
   if strings.Contains(err.Error(),"yt-dlp metadata lookup failed"){result.Error="yt-dlp could not fetch this video's metadata. Link retained."}
   if kind=="private"{result.Error="Cannot play this video: it is private. Link retained."}
   if kind=="deleted"{result.Error="Video reportedly deleted by uploader; verifying before automatic removal."}
   slog.Warn("background YouTube metadata lookup failed","station",task.id,"reason",kind,"error",err)
  }
  // Persist status for automatic rechecks and display in Manage Music.
  evidence,confirmed,healthErr:=q.health.record(task.id,task.url,kind,time.Now().UTC())
  if healthErr!=nil{slog.Warn("cannot persist YouTube availability","error",healthErr)}
  result.Availability=kind
  result.DeletedConfirmations=evidence.DeletedConfirmations
  // Apply availability transitions immediately without resetting the
  // continuously encoded audio. This also restores newly public links.
  if healthErr==nil{
   libraryEditMu.Lock()
   if st:=q.stations[task.id];isStationVideo(st,task.url){
    if refreshErr:=syncStationLibrary(st);refreshErr!=nil{slog.Warn("failed to update available tracks","error",refreshErr)}
   }
   libraryEditMu.Unlock()
  }
  if confirmed&&healthErr==nil {
   libraryEditMu.Lock()
   st:=q.stations[task.id]
   if isStationVideo(st,task.url) {
    updated:=st.Config
    updated.YouTubeURLs=make([]string,0,len(st.Config.YouTubeURLs))
    for _,link:=range st.Config.YouTubeURLs{if link!=task.url{updated.YouTubeURLs=append(updated.YouTubeURLs,link)}}
    if saveErr:=config.UpdateManagedStation(task.id,updated);saveErr==nil{
     st.Config=updated
     if syncErr:=syncStationLibrary(st);syncErr!=nil{slog.Warn("deleted-video library refresh failed","station",task.id,"error",syncErr)}
     result.State="removed"
     result.Error="Uploader-deleted video removed after three spaced confirmations."
     slog.Info("auto-removed confirmed uploader-deleted YouTube entry","station",task.id)
    }else{slog.Warn("failed to save confirmed removal","error",saveErr)}
   }
   libraryEditMu.Unlock()
  }
  q.set(task.id,task.url,result)
 }
}

func(q *youtubeMetadataQueue) recheckKnownUnavailable(){
 // Low-frequency health checks do not spawn FFmpeg nor block requests.
 ticker:=time.NewTicker(time.Hour)
 defer ticker.Stop()
 for range ticker.C {
  for _,item:=range q.health.due(time.Now().UTC()){
   st:=q.stations[item[0]]
   if !isStationVideo(st,item[1]){continue}
   _,_=q.enqueue(item[0],item[1],false)
  }
 }
}
