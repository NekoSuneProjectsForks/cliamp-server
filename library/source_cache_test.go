package library

import (
 "path/filepath"
 "testing"
)

func TestYouTubeVideoIDNormalizesWatchAndShortLinks(t *testing.T){
 urls:=[]string{
  "https://www.youtube.com/watch?v=AMmBY2D9IJU",
  "https://youtu.be/AMmBY2D9IJU?t=10",
  "https://music.youtube.com/watch?v=AMmBY2D9IJU&list=none",
  "https://youtube.com/watch?v=AMmBY2D9IJU",
 }
 for _,value:=range urls{if got:=YouTubeVideoID(value);got!="AMmBY2D9IJU"{t.Fatalf("id from %q = %q",value,got)}}
 for _,value:=range []string{"https://example.com/watch?v=AMmBY2D9IJU","http://youtube.com/watch?v=AMmBY2D9IJU","https://youtube.com/watch?v=../bad","https://youtube.com/playlist?list=X"}{
  if got:=YouTubeVideoID(value);got!=""{t.Fatalf("unsafe url %q became %q",value,got)}
 }
}
func TestYouTubeSourceCacheReusedAcrossURLsAndStations(t *testing.T){
 t.Setenv("CLIAMP_SOURCE_CACHE_FILE",filepath.Join(t.TempDir(),"cached-videos.sqlite"))
 t.Setenv("CLIAMP_TRACK_META_FILE",filepath.Join(t.TempDir(),"manual.json"))
 first:="https://www.youtube.com/watch?v=AMmBY2D9IJU"
 alternate:="https://youtu.be/AMmBY2D9IJU"
 entry:=SourceCacheEntry{Title:"MAYHEM",Artist:"THIRST",OriginalTitle:"THIRST - MAYHEM [Drift Phonk | Monstercat]",ArtworkChecked:true}
 if err:=SaveCachedYouTube(first,entry);err!=nil{t.Fatal(err)}
 // Simulate a process restart: close the SQLite handle and reopen from disk.
 sourceCacheMu.Lock()
 if cacheDB!=nil{_ = cacheDB.Close();cacheDB=nil;cacheDBPath=""}
 sourceCacheMu.Unlock()
 got,found,err:=CachedYouTube(alternate)
 if err!=nil||!found||got.Title!="MAYHEM"||!got.ArtworkChecked{t.Fatalf("cache miss %+v %v",got,err)}
 path:="youtube:"+alternate
 tracks:=[]Track{{Path:path,Title:YouTubePendingTitle}}
 if err:=ApplySourceCache("second-radio",tracks);err!=nil{t.Fatal(err)}
 if tracks[0].Artist!="THIRST"||tracks[0].Title!="MAYHEM"{t.Fatalf("missing cached artist/title %+v",tracks[0])}
 if err:=SaveMetadata("second-radio",path,Metadata{Title:"Custom title",Artist:"Custom artist"});err!=nil{t.Fatal(err)}
 tracks=[]Track{{Path:path,Title:YouTubePendingTitle}}
 if err:=ApplySourceCache("second-radio",tracks);err!=nil{t.Fatal(err)}
 if err:=ApplyMetadata("second-radio",tracks);err!=nil{t.Fatal(err)}
 if tracks[0].Title!="Custom title"{t.Fatalf("manual edit lost: %+v",tracks[0])}
}

func TestLateYouTubeMetadataUsesSavedValuesWithoutRestart(t *testing.T){
 t.Setenv("CLIAMP_SOURCE_CACHE_FILE",filepath.Join(t.TempDir(),"shared.sqlite"))
 t.Setenv("CLIAMP_TRACK_META_FILE",filepath.Join(t.TempDir(),"station.json"))
 const link="https://www.youtube.com/watch?v=AMmBY2D9IJU"
 stale:=Track{Path:"youtube:"+link,Title:YouTubePendingTitle}
 if got:=ResolveLatestYouTubeTrack("monstercat-music",stale);got.Title!=YouTubePendingTitle{
  t.Fatalf("unexpected initial title: %+v",got)
 }
 if err:=SaveCachedYouTube(link,SourceCacheEntry{Title:"MAYHEM",Artist:"THIRST"});err!=nil{t.Fatal(err)}
 got:=ResolveLatestYouTubeTrack("monstercat-music",stale)
 if got.Title!="MAYHEM"||got.Artist!="THIRST"||got.Path!=stale.Path{t.Fatalf("cached title did not take effect: %+v",got)}
 if err:=SaveMetadata("monstercat-music",stale.Path,Metadata{Title:"MAYHEM (Edit)",Artist:"THIRST"});err!=nil{t.Fatal(err)}
 got=ResolveLatestYouTubeTrack("monstercat-music",stale)
 if got.Title!="MAYHEM (Edit)"||got.Artist!="THIRST"{t.Fatalf("manual override was not used: %+v",got)}
}
