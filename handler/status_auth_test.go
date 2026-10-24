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
