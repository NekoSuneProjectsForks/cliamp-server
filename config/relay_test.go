package config

import (
 "os"
 "strings"
 "testing"
)

func TestIcecastRelayValidation(t *testing.T) {
 cfg := Defaults()
 cfg.Stations = map[string]StationConfig{"radio": {
  Path: t.TempDir(),
  Relays: []RelayConfig{{Type:"icecast2",Host:"icecast.example.com",Port:8000,Mount:"/stream",PasswordEnv:"CLIAMP_TEST_SOURCE_PASSWORD"}},
 }}
 t.Setenv("CLIAMP_TEST_SOURCE_PASSWORD","test-secret")
 if err:=cfg.Validate();err!=nil { t.Fatalf("expected valid Icecast relay, got %v",err) }
 st:=cfg.Stations["radio"]
 st.Relays[0].Type = "shoutcast2"
 cfg.Stations["radio"]=st
 if err:=cfg.Validate();err==nil || !strings.Contains(err.Error(),"not supported") { t.Fatalf("expected unsupported relay error, got %v",err) }
 st.Relays[0].Type="icecast2"
 st.Relays[0].Mount="invalid"
 cfg.Stations["radio"]=st
 if err:=cfg.Validate();err==nil {t.Fatal("expected invalid mount rejection")}
 st.Relays[0].Mount="/stream"
 cfg.Stations["radio"]=st
 os.Unsetenv("CLIAMP_TEST_SOURCE_PASSWORD")
 if err:=cfg.Validate();err==nil {t.Fatal("expected missing password rejection")}
}
