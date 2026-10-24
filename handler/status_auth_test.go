package handler

import (
 "net/http"
 "net/http/httptest"
 "testing"
)

func TestStatusAdminRequired(t *testing.T) {
 cases:=[]struct{name string; password string; authorization string; want int}{
  {"unset password","", "",http.StatusServiceUnavailable},
  {"missing bearer","topsecret","",http.StatusUnauthorized},
  {"wrong bearer","topsecret","Bearer wrong",http.StatusUnauthorized},
  {"correct bearer","topsecret","Bearer topsecret",http.StatusOK},
 }
 for _,tc:=range cases{
  t.Run(tc.name,func(t *testing.T){
   req:=httptest.NewRequest(http.MethodGet,"/status",nil)
   if tc.authorization!="" {req.Header.Set("Authorization",tc.authorization)}
   recorder:=httptest.NewRecorder()
   if !requireStatusAdmin(recorder,req,tc.password) && tc.want==http.StatusOK{t.Fatal("valid token was rejected")}
   if tc.want!=http.StatusOK && recorder.Code!=tc.want{t.Fatalf("got %d want %d",recorder.Code,tc.want)}
  })
 }
}

func TestStatusLoopbackPolicy(t *testing.T) {
 tests:=[]struct{ name,remote,host string; headers map[string]string; want bool }{
  {"localhost direct","127.0.0.1:1234","127.0.0.1:8000",nil,true},
  {"ipv6 direct","[::1]:1234","[::1]:8000",nil,true},
  {"localhost name","127.0.0.1:1234","localhost:8000",nil,true},
  {"remote","203.0.113.15:3456","127.0.0.1:8000",nil,false},
  {"proxy public host","127.0.0.1:4444","radio.example",nil,false},
  {"proxy forward","127.0.0.1:4444","127.0.0.1:8000",map[string]string{"X-Forwarded-For":"203.0.113.15"},false},
  {"proxy forwarded","127.0.0.1:4444","127.0.0.1:8000",map[string]string{"Forwarded":"for=203.0.113.15"},false},
  {"private LAN","192.168.1.2:9999","localhost:8000",nil,false},
 }
 for _,tt:=range tests {t.Run(tt.name,func(t *testing.T){
  r:=httptest.NewRequest(http.MethodGet,"http://"+tt.host+"/status",nil)
  r.RemoteAddr=tt.remote
  for k,v:=range tt.headers {r.Header.Set(k,v)}
  if got:=localStatusRequest(r);got!=tt.want {t.Fatalf("localStatusRequest=%v, want %v",got,tt.want)}
 })}
}
