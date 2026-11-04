package server

import (
 "context"
 "errors"
 "sync/atomic"
 "path/filepath"
 "testing"
 "time"

 "cliamp-server/config"
 "cliamp-server/library"
 "cliamp-server/youtubemeta"
)
func TestMetadataQueueDoesNotBlockAndDeduplicates(t *testing.T){
 t.Setenv("CLIAMP_TRACK_META_FILE",filepath.Join(t.TempDir(),"metadata.json"))
 t.Setenv("CLIAMP_YOUTUBE_HEALTH_FILE",filepath.Join(t.TempDir(),"youtube-health.json"))
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
 t.Setenv("CLIAMP_YOUTUBE_HEALTH_FILE",filepath.Join(t.TempDir(),"youtube-health.json"))
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

func TestYouTubeVideoIDCacheAvoidsSecondExtractorCall(t *testing.T){
 t.Setenv("CLIAMP_TRACK_META_FILE",filepath.Join(t.TempDir(),"track-metadata.json"))
 t.Setenv("CLIAMP_SOURCE_CACHE_FILE",filepath.Join(t.TempDir(),"youtube-cache.sqlite"))
 t.Setenv("CLIAMP_YOUTUBE_HEALTH_FILE",filepath.Join(t.TempDir(),"youtube-health.json"))
 t.Setenv("CLIAMP_YOUTUBE_ARTWORK_DIR",t.TempDir())
 a:="https://www.youtube.com/watch?v=AMmBY2D9IJU"
 b:="https://youtu.be/AMmBY2D9IJU"
 stations:=map[string]*Station{
  "one":{ID:"one",Config:config.StationConfig{Source:"youtube",YouTubeURLs:[]string{a}}},
  "two":{ID:"two",Config:config.StationConfig{Source:"youtube",YouTubeURLs:[]string{b}}},
 }
 var calls atomic.Int32
 q:=newYouTubeMetadataQueue(stations,func(context.Context,string)(youtubemeta.Metadata,error){
  calls.Add(1)
  return youtubemeta.Metadata{Title:"MAYHEM",Artist:"THIRST",RawTitle:"THIRST - MAYHEM [Monstercat]",Availability:"public"},nil
 })
 wait:=func(id,url string){
  t.Helper()
  if _,err:=q.enqueue(id,url,false);err!=nil{t.Fatal(err)}
  deadline:=time.After(4*time.Second)
  for{
   result,ok:=q.status(id,url)
   if ok&&result.State=="completed"{return}
   if ok&&result.State=="failed"{t.Fatalf("metadata worker failed %+v",result)}
   select{case <-deadline:t.Fatalf("timeout waiting for %s: %+v",id,result);case <-time.After(10*time.Millisecond):}
  }
 }
 wait("one",a)
 wait("two",b)
 if got:=calls.Load();got!=1{t.Fatalf("yt-dlp queried %d times for same YouTube ID",got)}
 fromSecond,exists,err:=library.MetadataFor("two","youtube:"+b)
 if err!=nil||!exists||fromSecond.Title!="MAYHEM"||fromSecond.Artist!="THIRST"{t.Fatalf("cached second station metadata missing: %+v %v",fromSecond,err)}
}

func TestCachedLegacyMetadataOnlyFetchesMissingArtwork(t *testing.T){
 t.Setenv("CLIAMP_TRACK_META_FILE",filepath.Join(t.TempDir(),"legacy.json"))
 t.Setenv("CLIAMP_SOURCE_CACHE_FILE",filepath.Join(t.TempDir(),"youtube-source.sqlite"))
 t.Setenv("CLIAMP_YOUTUBE_HEALTH_FILE",filepath.Join(t.TempDir(),"health.json"))
 t.Setenv("CLIAMP_YOUTUBE_ARTWORK_DIR",t.TempDir())
 const link="https://www.youtube.com/watch?v=AMmBY2D9IJU"
 if err:=library.SaveMetadata("radio","youtube:"+link,library.Metadata{Artist:"THIRST",Title:"MAYHEM"});err!=nil{t.Fatal(err)}
 if err:=library.SaveCachedYouTube(link,library.SourceCacheEntry{Artist:"THIRST",Title:"MAYHEM"});err!=nil{t.Fatal(err)}
 st:=&Station{ID:"radio",Config:config.StationConfig{Source:"youtube",YouTubeURLs:[]string{link}}}
 var extractorCalls atomic.Int32
 q:=newYouTubeMetadataQueue(map[string]*Station{"radio":st},func(context.Context,string)(youtubemeta.Metadata,error){
  extractorCalls.Add(1)
  return youtubemeta.Metadata{},errors.New("should not fetch yt-dlp again")
 })
 var artCalls atomic.Int32
 q.artwork=func(context.Context,string,string,string)(string,error){artCalls.Add(1);return "",nil}
 if _,err:=q.enqueue("radio",link,false);err!=nil{t.Fatal(err)}
 deadline:=time.After(3*time.Second)
 for{
  result,_:=q.status("radio",link)
  if result.State=="completed"{break}
  if result.State=="failed"{t.Fatalf("background failed %+v",result)}
  select{case <-deadline:t.Fatal("cached refresh took too long");case <-time.After(10*time.Millisecond):}
 }
 if extractorCalls.Load()!=0{t.Fatal("yt-dlp called despite cached video title")}
 if artCalls.Load()!=1{t.Fatalf("artwork lookup called %d times",artCalls.Load())}
 saved,ok,err:=library.CachedYouTube(link)
 if err!=nil||!ok||!saved.ArtworkChecked{t.Fatalf("artwork status not persisted %+v %v",saved,err)}
}
