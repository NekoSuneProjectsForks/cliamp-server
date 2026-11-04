package server

import (
 "context"
 "errors"
 "path/filepath"
 "testing"
 "time"

 "cliamp-server/config"
 "cliamp-server/library"
 "cliamp-server/youtubemeta"
)
func TestMetadataQueueDoesNotBlockAndDeduplicates(t *testing.T){
 t.Setenv("CLIAMP_TRACK_META_FILE",filepath.Join(t.TempDir(),"metadata.json"))
 const url="https://www.youtube.com/watch?v=AMmBY2D9IJU"
 st:=&Station{ID:"radio",Config:config.StationConfig{Source:"youtube",YouTubeURLs:[]string{url}}}
 release:=make(chan struct{})
 q:=newYouTubeMetadataQueue(map[string]*Station{"radio":st},
  func(ctx context.Context,videoURL string)(youtubemeta.Metadata,error){
   select{case <-release:return youtubemeta.Metadata{Artist:"THIRST",Title:"MAYHEM",RawTitle:"THIRST - MAYHEM [Monstercat]"},nil;case <-ctx.Done():return youtubemeta.Metadata{},ctx.Err()}
  })
 start:=time.Now()
 first,err:=q.enqueue("radio",url,false)
 if err!=nil||time.Since(start)>time.Second{t.Fatalf("enqueue blocked: %+v %v",first,err)}
 second,err:=q.enqueue("radio",url,false)
 if err!=nil||!(second.State=="queued"||second.State=="running"){t.Fatalf("duplicate scheduled: %+v %v",second,err)}
 close(release)
 deadline:=time.After(3*time.Second)
 for {
  current,_:=q.status("radio",url)
  if current.State=="completed"{
   saved,exists,err:=library.MetadataFor("radio","youtube:"+url)
   if err!=nil||!exists||saved.Title!="MAYHEM"{t.Fatalf("metadata not stored %+v %v",saved,err)}
   break
  }
  select{case <-deadline:t.Fatalf("background metadata did not finish: %+v",current);case <-time.After(10*time.Millisecond):}
 }
}
func TestMetadataQueueReportsErrorWithoutBlocking(t *testing.T){
 const url="https://www.youtube.com/watch?v=AMmBY2D9IJU"
 st:=&Station{ID:"radio",Config:config.StationConfig{Source:"youtube",YouTubeURLs:[]string{url}}}
 q:=newYouTubeMetadataQueue(map[string]*Station{"radio":st},
  func(context.Context,string)(youtubemeta.Metadata,error){return youtubemeta.Metadata{},errors.New("video unavailable")})
 if _,err:=q.enqueue("radio",url,false);err!=nil{t.Fatal(err)}
 deadline:=time.After(2*time.Second)
 for{
  status,_:=q.status("radio",url)
  if status.State=="failed"{
   if status.Error==""{t.Fatal("missing error status")}
   return
  }
  select{case <-deadline:t.Fatalf("status never failed: %+v",status);case <-time.After(10*time.Millisecond):}
 }
}
