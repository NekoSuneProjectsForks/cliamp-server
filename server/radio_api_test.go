package server

import (
 "encoding/json"
 "net/http"
 "net/http/httptest"
 "strings"
 "testing"

 "cliamp-server/broadcast"
 "cliamp-server/config"
)

func TestGameRadioAPI(t *testing.T) {
 cfg:=config.Defaults()
 cfg.Stations=map[string]config.StationConfig{
  "neffex":{Name:"NEFFEX",Genre:"Rock",VRChatHLS:true},
  "chill":{Name:"Chill",Genre:"Lofi"},
 }
 cfg.StationOrder=[]string{"neffex","chill"}
 stations:=map[string]*Station{}
 for id,sc:=range cfg.Stations {
  stations[id]=&Station{ID:id,Config:sc,Hub:broadcast.NewHub(id,nil,64,0)}
 }
 server:=New(cfg,stations,nil,nil)
 call:=func(path string)*httptest.ResponseRecorder{
  rec:=httptest.NewRecorder()
  req:=httptest.NewRequest("GET","https://radio.nekosunevr.co.uk"+path,nil)
  server.httpServer.Handler.ServeHTTP(rec,req)
  return rec
 }
 rec:=call("/api/v1/radios")
 if rec.Code!=http.StatusOK{t.Fatalf("status %d: %s",rec.Code,rec.Body.String())}
 var list struct{
  Count int `json:"count"`
  Radios []GameRadio `json:"radios"`
 }
 if err:=json.Unmarshal(rec.Body.Bytes(),&list);err!=nil{t.Fatal(err)}
 if list.Count!=2||len(list.Radios)!=2||list.Radios[0].ID!="neffex"{t.Fatalf("unexpected radios: %+v",list)}
 first:=list.Radios[0]
 if first.StreamURL!="https://radio.nekosunevr.co.uk/neffex/stream"{t.Fatal(first.StreamURL)}
 if first.VRChatHLSURL!="https://radio.nekosunevr.co.uk/neffex/vrchat/index.m3u8"{t.Fatal(first.VRChatHLSURL)}
 if first.PreferredURL!=first.VRChatHLSURL||first.Format!="hls"{t.Fatalf("bad VRChat preference: %+v",first)}
 if list.Radios[1].VRChatHLSURL!=""||list.Radios[1].Format!="mp3"{t.Fatal("HLS not opt-in")}
 if strings.Contains(rec.Body.String(),"/data/")||strings.Contains(rec.Body.String(),"secret"){t.Fatal("API leaked internal data")}
 if rec.Header().Get("Access-Control-Allow-Origin")!="*"{t.Fatal("game API missing CORS")}
 single:=call("/api/v1/radios/neffex")
 if single.Code!=200{t.Fatalf("single status %d",single.Code)}
 if call("/api/v1/radios/missing").Code!=404{t.Fatal("missing radio should be 404")}
 if call("/api/v1/radios?online_only=true").Code!=200{t.Fatal("online filter failed")}
}
