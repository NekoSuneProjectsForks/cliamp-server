package server

import (
 "bytes"
 "encoding/json"
 "net/http"
 "net/http/httptest"
 "testing"
 "time"

 "cliamp-server/broadcast"
 "cliamp-server/config"
 "cliamp-server/vrchathls"
)

func TestSharedLocalArtworkAcrossAllRadioAPIs(t *testing.T){
 oldDir:=vrchathls.AssetsDirectory
 vrchathls.AssetsDirectory=t.TempDir()
 t.Cleanup(func(){vrchathls.AssetsDirectory=oldDir})
 cfg:=config.Defaults()
 cfg.Stations=map[string]config.StationConfig{"neffex":{Name:"Neffex",VRChatHLS:true}}
 cfg.StationOrder=[]string{"neffex"}
 st:=&Station{ID:"neffex",Config:cfg.Stations["neffex"],Hub:broadcast.NewHub("neffex",nil,64,0)}
 srv:=New(cfg,map[string]*Station{"neffex":st},nil,nil)
 raw:=cdJewelCasePNG()
 if err:=vrchathls.SaveFallbackCover("neffex",raw);err!=nil{t.Fatal(err)}
 if _,_,err:=vrchathls.UpdateCurrentArt("neffex","","","",time.Time{});err!=nil{t.Fatal(err)}
 call:=func(path string)*httptest.ResponseRecorder{
  w:=httptest.NewRecorder()
  r:=httptest.NewRequest(http.MethodGet,"https://radio.nekosunevr.co.uk"+path,nil)
  srv.httpServer.Handler.ServeHTTP(w,r)
  return w
 }
 direct:=call("/api/neffex/art.png")
 if direct.Code!=200||direct.Header().Get("Content-Type")!="image/png"{t.Fatalf("local art HTTP %d",direct.Code)}
 if !bytes.Equal(direct.Body.Bytes(),raw){t.Fatal("station API did not return local cached artwork")}
 original:=call("/neffex/art.png")
 if original.Code!=200||!bytes.Equal(original.Body.Bytes(),raw){t.Fatal("HLS public art endpoint returned different image")}
 single:=call("/api/neffex/nowplaying")
 var metadata PublicStationRadio
 if err:=json.Unmarshal(single.Body.Bytes(),&metadata);err!=nil{t.Fatal(err)}
 expected:="https://radio.nekosunevr.co.uk/api/neffex/art.png"
 if metadata.CurrentTrack.CoverURL!=expected{t.Fatalf("wrong station art URL: %s",metadata.CurrentTrack.CoverURL)}
 listing:=call("/api/public/radio")
 var list struct {Stations []PublicStationRadio `json:"stations"`}
 if err:=json.Unmarshal(listing.Body.Bytes(),&list);err!=nil{t.Fatal(err)}
 if len(list.Stations)!=1||list.Stations[0].CurrentTrack.CoverURL!=expected{t.Fatal("directory did not use shared art URL")}
 game:=call("/api/v1/radios/neffex")
 var radio GameRadio
 if err:=json.Unmarshal(game.Body.Bytes(),&radio);err!=nil{t.Fatal(err)}
 if radio.ArtworkURL!=expected{t.Fatalf("game radio API not sharing cached art: %s",radio.ArtworkURL)}
}
