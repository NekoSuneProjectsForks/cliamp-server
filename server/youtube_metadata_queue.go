package server

import (
 "context"
 "fmt"
 "log/slog"
 "sync"
 "time"

 "cliamp-server/library"
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
}
type youtubeMetadataStatus struct {
 State string `json:"state"`
 Error string `json:"error,omitempty"`
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
  jobs:make(map[string]youtubeMetadataStatus),pending:make(chan youtubeMetadataTask,512),stations:stations,lookup:youtubemeta.Lookup,
 }
 if len(customLookup)>0&&customLookup[0]!=nil{q.lookup=customLookup[0]}
 go q.run()
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
  meta,err:=q.lookup(ctx,task.url)
  cancel()
  if err==nil {
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
     if err==nil{err=syncStationLibrary(st)}
    }
   }
   libraryEditMu.Unlock()
  }
  result:=youtubeMetadataStatus{State:"completed",Artist:meta.Artist,Title:meta.Title,UpdatedAt:time.Now().UTC()}
  if err!=nil{
   result.State="failed"
   result.Error="YouTube metadata unavailable; retry or edit the track manually"
   slog.Warn("background YouTube metadata lookup failed","station",task.id,"error",err)
  }
  q.set(task.id,task.url,result)
 }
}
