package config

import (
 "path/filepath"
 "testing"
)

func TestYouTubeStationValidation(t *testing.T) {
 cfg:=Defaults()
 cfg.Stats.DBPath=""
 cfg.Stations=map[string]StationConfig{"videos":{Source:"youtube",Name:"Videos"}}
 if err:=cfg.Validate();err!=nil{t.Fatalf("empty YouTube station should be allowed: %v",err)}
 st:=cfg.Stations["videos"]
 st.YouTubeURLs=[]string{"https://www.youtube.com/watch?v=dQw4w9WgXcQ"}
 cfg.Stations["videos"]=st
 if err:=cfg.Validate();err!=nil{t.Fatalf("valid YouTube video rejected: %v",err)}
 st.YouTubeURLs=[]string{"https://localhost/watch?v=dQw4w9WgXcQ"}
 cfg.Stations["videos"]=st
 if err:=cfg.Validate();err==nil{t.Fatal("unexpected acceptance of non-YouTube host")}
}
func TestManagedStationReadback(t *testing.T) {
 t.Setenv("CLIAMP_STATIONS_FILE",filepath.Join(t.TempDir(),"stations.json"))
 if err:=CreateManagedStation("test-radio",StationConfig{Name:"Test Radio",Source:"youtube"});err!=nil{t.Fatal(err)}
 stations,err:=ManagedStations()
 if err!=nil{t.Fatal(err)}
 if got:=stations["test-radio"].Source;got!="youtube"{t.Fatalf("source=%q",got)}
 if ValidStationID("../unsafe"){t.Fatal("path-like station ID accepted")}
 if err:=DeleteManagedStation("test-radio");err!=nil{t.Fatal(err)}
 after,err:=ManagedStations()
 if err!=nil{t.Fatal(err)}
 if _,ok:=after["test-radio"];ok{t.Fatal("deleted pending station still listed")}
}
