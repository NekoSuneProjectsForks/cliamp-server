package server

import (
 "encoding/json"
 "errors"
 "os"
 "path/filepath"
 "sync"
 "time"
)

// videoHealthStore persists provider availability independently of metadata.
// A private/unlisted video is never automatically deleted.
type videoHealthStore struct {
 mu sync.Mutex
 entries map[string]videoHealth
 path string
}
type videoHealth struct {
 State string `json:"state"`
 FirstDeleted time.Time `json:"first_deleted,omitempty"`
 LastChecked time.Time `json:"last_checked"`
 DeletedConfirmations int `json:"deleted_confirmations,omitempty"`
}
const deletionConfirmationSpacing=24*time.Hour
const deletionConfirmationCount=3

func newVideoHealthStore(path string)*videoHealthStore{
 h:=&videoHealthStore{entries:map[string]videoHealth{},path:path}
 if b,err:=os.ReadFile(path);err==nil{if json.Unmarshal(b,&h.entries)!=nil{h.entries=map[string]videoHealth{}}}
 return h
}
func videoHealthFile()string{
 if path:=os.Getenv("CLIAMP_YOUTUBE_HEALTH_FILE");path!=""{return path}
 return "/data/youtube-video-health.json"
}
func(h *videoHealthStore) get(id,url string)videoHealth{
 h.mu.Lock();defer h.mu.Unlock()
 return h.entries[metadataKey(id,url)]
}
func(h *videoHealthStore) record(id,url,kind string,now time.Time)(videoHealth,bool,error){
 h.mu.Lock()
 defer h.mu.Unlock()
 key:=metadataKey(id,url)
 before:=h.entries[key]
 value:=videoHealth{State:kind,LastChecked:now}
 if kind=="deleted" {
  if before.State=="deleted" {
   value.FirstDeleted=before.FirstDeleted
   value.DeletedConfirmations=before.DeletedConfirmations
   if value.FirstDeleted.IsZero(){value.FirstDeleted=now}
   if now.Sub(before.LastChecked)>=deletionConfirmationSpacing{value.DeletedConfirmations++}
  } else {
   value.FirstDeleted=now
   value.DeletedConfirmations=1
  }
 }
 h.entries[key]=value
 if err:=h.persist();err!=nil{
  h.entries[key]=before
  return value,false,err
 }
 // Three explicit uploader-removed responses, separated by >=24h.
 // Never remove on transient errors, permission changes or ambiguous errors.
 confirmed:=kind=="deleted"&&value.DeletedConfirmations>=deletionConfirmationCount&&
  now.Sub(value.FirstDeleted)>=2*deletionConfirmationSpacing
 return value,confirmed,nil
}
func(h *videoHealthStore) persist()error{
 if err:=os.MkdirAll(filepath.Dir(h.path),0700);err!=nil{return err}
 b,err:=json.Marshal(h.entries);if err!=nil{return err}
 tmp,err:=os.CreateTemp(filepath.Dir(h.path),".youtube-health-*");if err!=nil{return err}
 defer os.Remove(tmp.Name())
 if err:=tmp.Chmod(0600);err!=nil{tmp.Close();return err}
 if _,err:=tmp.Write(b);err!=nil{tmp.Close();return err}
 if err:=tmp.Sync();err!=nil{tmp.Close();return err}
 if err:=tmp.Close();err!=nil{return err}
 return os.Rename(tmp.Name(),h.path)
}
func(h *videoHealthStore) due(now time.Time)[][2]string{
 h.mu.Lock();defer h.mu.Unlock()
 result:=make([][2]string,0)
 for key,entry:=range h.entries{
  if entry.State!="deleted"&&entry.State!="private"{continue}
  if now.Sub(entry.LastChecked)<deletionConfirmationSpacing{continue}
  sep:= -1
  for i:=0;i<len(key);i++{if key[i]==0{sep=i;break}}
  if sep<=0||sep+1>=len(key){continue}
  result=append(result,[2]string{key[:sep],key[sep+1:]})
 }
 return result
}
func(h *videoHealthStore) forget(id,url string)error{
 h.mu.Lock();defer h.mu.Unlock()
 key:=metadataKey(id,url)
 delete(h.entries,key)
 return h.persist()
}
var _=errors.New
