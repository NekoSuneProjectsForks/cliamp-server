package server

import (
 "bytes"
 "context"
 "testing"
 "time"

 "cliamp-server/broadcast"
 "cliamp-server/config"
 "cliamp-server/vrchathls"
)

func TestBackgroundArtworkSyncBuildsLocalArtWithoutHLS(t *testing.T){
 old:=vrchathls.AssetsDirectory
 vrchathls.AssetsDirectory=t.TempDir()
 t.Cleanup(func(){vrchathls.AssetsDirectory=old})
 station:=&Station{ID:"neffex",Config:config.StationConfig{Name:"Neffex",VRChatHLS:false},Hub:broadcast.NewHub("neffex",nil,64,0)}
 ctx,cancel:=context.WithCancel(context.Background())
 done:=make(chan struct{})
 go func(){defer close(done);syncOneStationArtwork(ctx,"neffex",station,newArtworkLookup())}()
 defer func(){
  cancel()
  select{case <-done:case <-time.After(time.Second):t.Error("background artwork worker did not stop")}
 }()
 deadline:=time.Now().Add(2*time.Second)
 for time.Now().Before(deadline){
  art,err:=vrchathls.ReadCurrentArt("neffex")
  if err==nil {
   if !bytes.Equal(art,cdJewelCasePNG()){t.Fatal("background worker did not create shared CD fallback")}
   return
  }
  time.Sleep(10*time.Millisecond)
 }
 t.Fatal("station art.png was not populated without HLS")
}
