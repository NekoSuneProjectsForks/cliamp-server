package server

import (
 "net"
 "net/http"
 "net/http/httptest"
 "testing"
)

func TestExternalNowPlayingRejectsUntrustedStationID(t *testing.T){
 service:=newRadioNowPlaying()
 for _,id:=range []string{"", "http://127.0.0.1/admin","../../private","neffex"}{
  req:=httptest.NewRequest(http.MethodGet,"/api/public/discover/nowplaying?id="+id,nil)
  resp:=httptest.NewRecorder()
  service.serve(resp,req)
  if resp.Code!=http.StatusBadRequest{t.Fatalf("id %q: status %d",id,resp.Code)}
 }
}
func TestRadioIPValidation(t *testing.T){
 for _,address:=range []string{"127.0.0.1","10.2.3.4","192.168.1.10","169.254.169.254","100.100.100.200","198.51.100.2","203.0.113.2","::1","fc00::1"} {
  if isPublicRadioIP(net.ParseIP(address)){t.Fatalf("private/reserved IP allowed: %s",address)}
 }
 for _,address:=range []string{"8.8.8.8","1.1.1.1","203.119.0.1"}{
  if !isPublicRadioIP(net.ParseIP(address)){t.Fatalf("public address blocked: %s",address)}
 }
}
