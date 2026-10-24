package config

import (
 "os"
 "path/filepath"
 "testing"
)

func TestManagedStationsPersistence(t *testing.T){
 file:=filepath.Join(t.TempDir(),"stations.json")
 t.Setenv("CLIAMP_STATIONS_FILE",file)
 if err:=CreateManagedStation("s3-music",StationConfig{Name:"S3 Music",S3:S3Config{Endpoint:"https://s3.example.org",Bucket:"radio"}});err!=nil{t.Fatal(err)}
 a:=Defaults();a.Stations=map[string]StationConfig{"local":{Name:"Local",Path:"/music/local"}}
 a.StationOrder=[]string{"local"}
 if err:=a.LoadStationOverrides();err!=nil{t.Fatal(err)}
 if _,ok:=a.Stations["s3-music"];!ok{t.Fatal("new station not loaded")}
 if err:=DeleteManagedStation("local");err!=nil{t.Fatal(err)}
 if err:=a.LoadStationOverrides();err!=nil{t.Fatal(err)}
 if _,ok:=a.Stations["local"];ok{t.Fatal("deleted local station still present")}
 if _,err:=os.Stat(file);err!=nil{t.Fatal(err)}
}
