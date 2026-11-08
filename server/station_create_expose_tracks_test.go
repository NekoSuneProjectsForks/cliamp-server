package server

import (
 "bytes"
 "encoding/json"
 "net/http"
 "net/http/httptest"
 "path/filepath"
 "testing"

 "cliamp-server/broadcast"
 "cliamp-server/config"
)

func TestCreateStationDefaultsToPublicPlaylistForYouTube(t *testing.T) {
 t.Setenv("CLIAMP_STATIONS_FILE",filepath.Join(t.TempDir(),"stations.json"))
 cfg:=config.Defaults()
 cfg.Admin.Password="create-test-password"
 cfg.Stations=map[string]config.StationConfig{"existing":{Name:"Existing",ExposeTracks:false}}
 cfg.StationOrder=[]string{"existing"}
 stations:=map[string]*Station{"existing":{ID:"existing",Hub:broadcast.NewHub("existing",nil,64,0),Config:cfg.Stations["existing"]}}
 srv:=New(cfg,stations,nil,nil)
 create:=func(id string,enabled *bool) {
  t.Helper()
  payload:=map[string]any{
   "id":id,"name":"YouTube "+id,"source":"youtube",
   "youtube_urls":[]string{"https://www.youtube.com/watch?v=abcDEF12345"},
  }
  if enabled!=nil{payload["expose_tracks"]=*enabled}
  encoded,_:=json.Marshal(payload)
  r:=httptest.NewRequest(http.MethodPost,"/api/admin/stations",bytes.NewReader(encoded))
  r.Header.Set("Authorization","Bearer create-test-password")
  w:=httptest.NewRecorder()
  srv.httpServer.Handler.ServeHTTP(w,r)
  if w.Code!=http.StatusOK{t.Fatalf("create %s returned %d: %s",id,w.Code,w.Body.String())}
 }
 create("neffex",nil)
 disable:=false
 create("private-yt",&disable)
 values,err:=config.ManagedStations()
 if err!=nil{t.Fatal(err)}
 if !values["neffex"].ExposeTracks{t.Fatal("YouTube station did not default to public on-demand playlists")}
 if values["private-yt"].ExposeTracks{t.Fatal("explicitly disabled YouTube track listing was ignored")}
 if cfg.Stations["existing"].ExposeTracks{t.Fatal("existing station privacy setting must not change")}
 reload:=config.Defaults()
 reload.Stations=map[string]config.StationConfig{}
 if err:=reload.LoadStationOverrides();err!=nil{t.Fatal(err)}
 if !reload.Stations["neffex"].ExposeTracks{t.Fatal("expose_tracks did not persist across restart")}
}
