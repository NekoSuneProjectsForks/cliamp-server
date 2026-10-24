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
