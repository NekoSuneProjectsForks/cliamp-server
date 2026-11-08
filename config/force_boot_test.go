package config

import (
 "os"
 "path/filepath"
 "strings"
 "testing"
)

func TestBootForcesPlaylistAfterManagedOverrides(t *testing.T) {
 t.Setenv("CLIAMP_STATIONS_FILE",filepath.Join(t.TempDir(),"stations.json"))
 t.Setenv("XDG_CONFIG_HOME",t.TempDir())
 cfgPath:=filepath.Join(t.TempDir(),"config.toml")
 if err:=os.WriteFile(cfgPath,[]byte(`[stats]
db_path = ""
[stations.neffex]
name = "Neffex Station"
source = "youtube"
expose_tracks = true
youtube_urls = ["https://www.youtube.com/watch?v=abcDEF12345"]
[stations.local]
name = "Local"
path = "/music/local"
expose_tracks = false
`),0600);err!=nil{t.Fatal(err)}
 // Older dashboard state can overwrite TOML, reverting expose_tracks.
 if err:=CreateManagedStation("neffex",StationConfig{Name:"Neffex Station",Source:"youtube",YouTubeURLs:[]string{"https://www.youtube.com/watch?v=abcDEF12345"},ExposeTracks:false});err!=nil{t.Fatal(err)}
 cfg,err:=Load(cfgPath)
 if err!=nil{t.Fatal(err)}
 if cfg.Stations["neffex"].ExposeTracks{t.Fatal("fixture did not reproduce managed override conflict")}
 if err:=cfg.EnablePublicMusicOnBoot();err!=nil{t.Fatal(err)}
 for id,station:=range cfg.Stations{
  if !station.ExposeTracks{t.Errorf("station %s still has public tracks disabled",id)}
 }
 if cfg.Stats.DBPath==""{t.Fatal("statistics database must be enabled on boot")}
 if info,err:=os.Stat(filepath.Dir(cfg.Stats.DBPath));err!=nil||!info.IsDir(){t.Fatalf("stats parent not accessible: %v",err)}
 // Runtime enforcement must not mutate saved files or require editing admin.
 managed,err:=ManagedStations()
 if err!=nil{t.Fatal(err)}
 if managed["neffex"].ExposeTracks{t.Fatal("force-boot must not silently rewrite saved explicit preferences")}
 raw,err:=os.ReadFile(cfgPath)
 if err!=nil||!strings.Contains(string(raw),"expose_tracks = false"){t.Fatal("manual TOML was unexpectedly changed")}
 if err:=cfg.EnablePublicMusicOnBoot();err!=nil{t.Fatal("boot policy must be idempotent: ",err)}
}

func TestBootKeepsExplicitPersistentStatsDB(t *testing.T){
 path:=filepath.Join(t.TempDir(),"existing.db")
 c:=Defaults()
 c.Stations=map[string]StationConfig{"neffex":{Name:"Neffex",ExposeTracks:false}}
 c.Stats.DBPath=path
 if err:=c.EnablePublicMusicOnBoot();err!=nil{t.Fatal(err)}
 if c.Stats.DBPath!=path{t.Fatal("configured persistent stats path unexpectedly replaced")}
 if !c.Stations["neffex"].ExposeTracks{t.Fatal("boot did not force expose_tracks")}
}
