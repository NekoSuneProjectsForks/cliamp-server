package server

import (
 "context"
 "errors"
 "sync"
 "time"

 "cliamp-server/library"
)

type ArtworkRefreshStatus struct {
 Station string `json:"station"`
 State string `json:"state"`
 Total int `json:"total"`
 Completed int `json:"completed"`
 Found int `json:"found"`
 Missing int `json:"missing"`
 Failed int `json:"failed"`
 Error string `json:"error,omitempty"`
 UpdatedAt time.Time `json:"updated_at"`
}
type artworkRefreshManager struct{
 mu sync.Mutex
 busy bool
 states map[string]ArtworkRefreshStatus
 covers *artworkLookup
}
func newArtworkRefreshManager(covers *artworkLookup)*artworkRefreshManager{
 return &artworkRefreshManager{states:make(map[string]ArtworkRefreshStatus),covers:covers}
}
func(m *artworkRefreshManager) status(id string)ArtworkRefreshStatus{
 m.mu.Lock();defer m.mu.Unlock()
 value,ok:=m.states[id]
 if !ok{return ArtworkRefreshStatus{Station:id,State:"idle"}}
 return value
}
func(m *artworkRefreshManager) update(status ArtworkRefreshStatus){
 m.mu.Lock();status.UpdatedAt=time.Now().UTC();m.states[status.Station]=status;m.mu.Unlock()
}
// start accepts a request instantly and runs a single global queue.
// No simultaneous multi-station catalogue scans are allowed on the Pi.
func(m *artworkRefreshManager) start(id string,station *Station)error{
 m.mu.Lock()
 if m.busy{m.mu.Unlock();return errors.New("another artwork refresh is running")}
 m.busy=true
 status:=ArtworkRefreshStatus{Station:id,State:"scanning",UpdatedAt:time.Now().UTC()}
 m.states[id]=status
 m.mu.Unlock()
 go func(){
  defer func(){m.mu.Lock();m.busy=false;m.mu.Unlock()}()
  // Snapshot the current playlist once; do not interrupt music output.
  ctx,cancel:=context.WithTimeout(context.Background(),2*time.Hour)
  defer cancel()
  libraryEditMu.Lock()
  tracks,err:=stationLibrary(ctx,station,id)
  libraryEditMu.Unlock()
  if err!=nil{
   status.State="failed";status.Error="Could not enumerate station library"
   m.update(status);return
  }
  // Deduplicate identical audio sources in the station's collection.
  seen:=make(map[string]bool)
  selected:=make([]library.Track,0,len(tracks))
  for _,track:=range tracks{
   identity:=library.ArtworkIdentity(id,track.Path,track.Artist,track.Title,track.Album)
   key:=identity.SourceType+"|"+identity.SourceKey+"|"+identity.Artist+"|"+identity.Title
   if seen[key]{continue}
   seen[key]=true
   selected=append(selected,track)
  }
  status.Total=len(selected)
  status.State="running";m.update(status)
  for _,track:=range selected{
   if ctx.Err()!=nil{status.State="failed";status.Error="Artwork refresh exceeded its time limit";break}
   if track.Artist==""||track.Title==""{
    status.Missing++
   }else{
    m.covers.forget(track.Artist,track.Title,track.Album)
    outcome,err:=cacheTrackArtwork(ctx,id,track,m.covers,true)
    if err!=nil{status.Failed++}else if outcome=="found"{status.Found++}else{status.Missing++}
   }
   status.Completed++
   m.update(status)
  }
  if status.State=="running"{status.State="completed"}
  m.update(status)
 }()
 return nil
}
