package server

import (
 "encoding/json"
 "bytes"
 "image/png"
 "net/http"
 "net/http/httptest"
 "strings"
 "testing"

 "cliamp-server/broadcast"
 "cliamp-server/config"
)

func TestStationSpecificPublicRadioAPI(t *testing.T){
 cfg:=config.Defaults()
 cfg.Stations=map[string]config.StationConfig{
  "neffex":{Name:"Neffex Station",Description:"Neffex 24/7",Genre:"neffex"},
  "chill":{Name:"Chill Station",Genre:"chill"},
 }
 cfg.StationOrder=[]string{"neffex","chill"}
 stations:=map[string]*Station{
  "neffex":{ID:"neffex",Config:cfg.Stations["neffex"],Hub:broadcast.NewHub("neffex",nil,64,0),TrackCount:40},
  "chill":{ID:"chill",Config:cfg.Stations["chill"],Hub:broadcast.NewHub("chill",nil,64,0)},
 }
 srv:=New(cfg,stations,nil,nil)
 request:=func(path string)*httptest.ResponseRecorder{
  rec:=httptest.NewRecorder()
  req:=httptest.NewRequest(http.MethodGet,"https://radio.nekosunevr.co.uk"+path,nil)
  srv.httpServer.Handler.ServeHTTP(rec,req)
  return rec
 }
 for _,path:=range []string{"/api/neffex","/api/neffex/","/api/neffex/nowplaying"}{
  rec:=request(path)
  if rec.Code!=http.StatusOK{t.Fatalf("%s returned %d: %s",path,rec.Code,rec.Body.String())}
  var station PublicStationRadio
  if err:=json.Unmarshal(rec.Body.Bytes(),&station);err!=nil{t.Fatal(err)}
  if station.ID!="neffex"||station.Name!="Neffex Station"||station.Tracks!=40||station.Genre!="neffex"||station.Description!="Neffex 24/7"||station.Stream!="/neffex/stream"{
   t.Fatalf("%s returned unexpected station: %+v",path,station)
  }
  if strings.Contains(rec.Body.String(),"listener_details")||strings.Contains(rec.Body.String(),"password")||strings.Contains(rec.Body.String(),"/data/"){
   t.Fatalf("private fields exposed by %s",path)
  }
  if rec.Header().Get("Access-Control-Allow-Origin")!="*"{t.Fatal("missing public CORS")}
  if rec.Header().Get("Cache-Control")!="no-store"{t.Fatal("nowplaying must not cache")}
  if strings.Contains(rec.Body.String(),"\"stations\""){t.Fatalf("%s should return a single station, not a list",path)}
 }
 for _,path:=range []string{"/api/missing","/api/missing/","/api/missing/nowplaying","/api/missing/art.png"}{
  if got:=request(path).Code;got!=404{t.Errorf("%s returned %d, want 404",path,got)}
 }
 art:=request("/api/neffex/art.png")
 original:=request("/neffex/art.png")
 if original.Code!=200 || !bytes.Equal(original.Body.Bytes(),art.Body.Bytes()){t.Fatalf("original artwork URL did not share CD fallback; status=%d",original.Code)}
 if art.Code!=200{t.Fatalf("expected CD-case placeholder when artwork is unavailable, got %d",art.Code)}
 if typ:=art.Header().Get("Content-Type");typ!="image/png"{t.Fatalf("CD fallback Content-Type=%q",typ)}
 if cc:=art.Header().Get("Cache-Control");cc!="no-store"{t.Fatalf("missing no-store: %q",cc)}
 picture,err:=png.Decode(bytes.NewReader(art.Body.Bytes()))
 if err!=nil{t.Fatalf("fallback not a valid PNG: %v",err)}
 if picture.Bounds().Dx()!=390 || picture.Bounds().Dy()!=390{t.Fatalf("unexpected placeholder size: %v",picture.Bounds())}
 if len(art.Body.Bytes())<1000{t.Fatal("placeholder PNG too small")}
 station:=request("/api/neffex/nowplaying")
 var current PublicStationRadio
 if err:=json.Unmarshal(station.Body.Bytes(),&current);err!=nil{t.Fatal(err)}
 if current.CurrentTrack.CoverURL!="https://radio.nekosunevr.co.uk/api/neffex/art.png"{
  t.Fatalf("expected fallback cover URL, got %q",current.CurrentTrack.CoverURL)
 }
 all:=request("/api/public/radio")
 if all.Code!=200 || !strings.Contains(all.Body.String(),"\"stations\""){t.Fatalf("legacy API broken: %d",all.Code)}
 game:=request("/api/v1/radios/neffex")
 if game.Code!=200{t.Fatalf("game radio API broken: %d",game.Code)}
}
