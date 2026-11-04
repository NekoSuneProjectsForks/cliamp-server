package library

import (
 "path/filepath"
 "testing"
)

func TestSavedMetadataSurvivesReload(t *testing.T){
 t.Setenv("CLIAMP_TRACK_META_FILE",filepath.Join(t.TempDir(),"metadata.json"))
 trackPath:="s3://station/music/test.mp3"
 err:=SaveMetadata("station",trackPath,Metadata{Title:"Edited title",Artist:"Edited artist",Album:"Edited album"})
 if err!=nil{t.Fatal(err)}
 tracks:=[]Track{{Path:trackPath,Title:"Original"}}
 if err:=ApplyMetadata("station",tracks);err!=nil{t.Fatal(err)}
 if tracks[0].Title!="Edited title"||tracks[0].Artist!="Edited artist"||tracks[0].Album!="Edited album"{t.Fatalf("metadata did not persist: %+v",tracks[0])}
 if err:=SaveMetadata("station",trackPath,Metadata{Title:""});err==nil{t.Fatal("empty title must fail")}
}

func TestYouTubeSourceTitleSurvivesManualEdit(t *testing.T){
 t.Setenv("CLIAMP_TRACK_META_FILE",filepath.Join(t.TempDir(),"metadata.json"))
 path:="youtube:https://www.youtube.com/watch?v=AMmBY2D9IJU"
 original:="THIRST - MAYHEM [Drift Phonk | Monstercat]"
 if err:=SaveMetadata("radio",path,Metadata{Title:"MAYHEM",Artist:"THIRST",SourceTitle:original});err!=nil{t.Fatal(err)}
 if err:=SaveMetadata("radio",path,Metadata{Title:"Mayhem Remix",Artist:"THIRST"});err!=nil{t.Fatal(err)}
 got,exists,err:=MetadataFor("radio",path)
 if err!=nil||!exists{t.Fatalf("metadata missing: %v",err)}
 if got.SourceTitle!=original||got.Title!="Mayhem Remix"{t.Fatalf("metadata lost: %+v",got)}
 tracks:=[]Track{{Path:path,Title:path}}
 if err:=ApplyMetadata("radio",tracks);err!=nil{t.Fatal(err)}
 if tracks[0].Artist!="THIRST"||tracks[0].Title!="Mayhem Remix"{t.Fatalf("bad override: %+v",tracks[0])}
}
