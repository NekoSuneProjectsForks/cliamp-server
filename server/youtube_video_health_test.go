package server

import (
 "path/filepath"
 "testing"
 "time"

 "cliamp-server/library"
)

func TestDeletedVideoNeedsThreeSpacedExplicitConfirmations(t *testing.T){
 path:=filepath.Join(t.TempDir(),"health.json")
 h:=newVideoHealthStore(path)
 start:=time.Date(2026,11,4,12,0,0,0,time.UTC)
 const url="https://www.youtube.com/watch?v=AMmBY2D9IJU"
 rec:=func(kind string,delta time.Duration)(videoHealth,bool){
  t.Helper()
  status,remove,err:=h.record("radio",url,kind,start.Add(delta))
  if err!=nil{t.Fatal(err)}
  return status,remove
 }
 if _,removed:=rec("deleted",0);removed{t.Fatal("deleted on first signal")}
 if status,removed:=rec("deleted",time.Hour);removed||status.DeletedConfirmations!=1{t.Fatalf("too early %+v",status)}
 if status,removed:=rec("deleted",24*time.Hour);removed||status.DeletedConfirmations!=2{t.Fatalf("too early %+v",status)}
 reloaded:=newVideoHealthStore(path)
 if reloaded.get("radio",url).DeletedConfirmations!=2{t.Fatal("health evidence not persisted")}
 if status,removed:=rec("deleted",48*time.Hour);!removed||status.DeletedConfirmations!=3{t.Fatalf("expected confirmed removal at 48h %+v",status)}
}
func TestTemporaryAndPrivateResultsNeverAutoDelete(t *testing.T){
 h:=newVideoHealthStore(filepath.Join(t.TempDir(),"health.json"))
 start:=time.Now().UTC()
 const url="https://www.youtube.com/watch?v=AMmBY2D9IJU"
 for _,state:=range []string{"private","unlisted","unknown","deleted","unknown","deleted","deleted"} {
  day:=time.Duration(1)
  if state=="deleted"{day=24}
  current,remove,err:=h.record("radio",url,state,start)
  if err!=nil{t.Fatal(err)}
  if remove{t.Fatalf("unexpected removal on %q %+v",state,current)}
  start=start.Add(time.Duration(day)*time.Hour)
 }
}
func TestPrivateHiddenButUnlistedStillPlayable(t *testing.T){
 t.Setenv("CLIAMP_YOUTUBE_HEALTH_FILE",filepath.Join(t.TempDir(),"health.json"))
 h:=newVideoHealthStore(videoHealthFile())
 private:="https://www.youtube.com/watch?v=AMmBY2D9IJU"
 unlisted:="https://www.youtube.com/watch?v=dQw4w9WgXcQ"
 now:=time.Now().UTC()
 if _,_,err:=h.record("radio",private,"private",now);err!=nil{t.Fatal(err)}
 if _,_,err:=h.record("radio",unlisted,"unlisted",now);err!=nil{t.Fatal(err)}
 tracks:=[]library.Track{{Path:"youtube:"+private,Title:"Private"},{Path:"youtube:"+unlisted,Title:"Unlisted"}}
 kept:=FilterPlayableYouTubeTracks("radio",tracks)
 if len(kept)!=1||kept[0].Path!="youtube:"+unlisted{t.Fatalf("unexpected filter: %+v",kept)}
 if _,_,err:=h.record("radio",private,"public",now.Add(time.Hour));err!=nil{t.Fatal(err)}
 kept=FilterPlayableYouTubeTracks("radio",tracks)
 if len(kept)!=2{t.Fatalf("private video not restored after public status: %+v",kept)}
}
