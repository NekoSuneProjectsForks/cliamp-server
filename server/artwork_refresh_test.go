package server

import (
 "encoding/json"
 "net/http"
 "net/http/httptest"
 "path/filepath"
 "testing"

 "cliamp-server/broadcast"
 "cliamp-server/config"
)

func TestArtworkAdminRefreshRequiresAuthentication(t *testing.T){
 t.Setenv("CLIAMP_ARTWORK_CACHE_DB",filepath.Join(t.TempDir(),"album-cache.sqlite"))
 cfg:=config.Defaults()
 cfg.Admin.Password="test-admin-key"
 cfg.Stations=map[string]config.StationConfig{"neffex":{Name:"Neffex",Source:"youtube"}}
 cfg.StationOrder=[]string{"neffex"}
 st:=&Station{ID:"neffex",Config:cfg.Stations["neffex"],Hub:broadcast.NewHub("neffex",nil,64,0)}
 srv:=New(cfg,map[string]*Station{"neffex":st},nil,nil)
 request:=func(method,path string,authorized bool)*httptest.ResponseRecorder{
  w:=httptest.NewRecorder()
  r:=httptest.NewRequest(method,path,nil)
  if authorized{r.Header.Set("Authorization","Bearer test-admin-key")}
  srv.httpServer.Handler.ServeHTTP(w,r)
  return w
 }
 for _,methodPath:=range [][2]string{
  {"GET","/api/admin/stations/neffex/artwork/cache"},
  {"POST","/api/admin/stations/neffex/artwork/refresh"},
 }{
  if got:=request(methodPath[0],methodPath[1],false).Code;got!=http.StatusUnauthorized{
   t.Fatalf("unauthorized %s returned %d",methodPath[1],got)
  }
 }
 view:=request("GET","/api/admin/stations/neffex/artwork/cache",true)
 if view.Code!=200{t.Fatalf("authorized database status %d: %s",view.Code,view.Body.String())}
 var status struct{Counts map[string]int `json:"counts"`}
 if err:=json.Unmarshal(view.Body.Bytes(),&status);err!=nil{t.Fatal(err)}
 if status.Counts==nil{t.Fatal("missing artwork database counters")}
 started:=request("POST","/api/admin/stations/neffex/artwork/refresh",true)
 if started.Code!=http.StatusAccepted{t.Fatalf("admin refresh %d: %s",started.Code,started.Body.String())}
 if got:=request("GET","/api/admin/stations/missing/artwork/cache",true).Code;got!=404{t.Fatalf("unknown station returned %d",got)}
}
