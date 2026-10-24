package library

import (
 "encoding/json"
 "errors"
 "os"
 "path/filepath"
 "strings"
 "sync"
)

type Metadata struct {
 Title string `json:"title"`
 Artist string `json:"artist"`
 Album string `json:"album"`
}
var metadataMu sync.Mutex
func metadataPath()string{if s:=os.Getenv("CLIAMP_TRACK_META_FILE");s!=""{return s};return "/data/track-metadata.json"}
func readMetadata()(map[string]map[string]Metadata,error){
 b,err:=os.ReadFile(metadataPath())
 if errors.Is(err,os.ErrNotExist){return map[string]map[string]Metadata{},nil}
 if err!=nil{return nil,err}
 var all map[string]map[string]Metadata
 if err:=json.Unmarshal(b,&all);err!=nil{return nil,err}
 if all==nil{all=map[string]map[string]Metadata{}}
 return all,nil
}
func ApplyMetadata(station string,tracks []Track)error{
 metadataMu.Lock();defer metadataMu.Unlock()
 all,err:=readMetadata();if err!=nil{return err}
 for i:=range tracks{
  if info,ok:=all[station][tracks[i].Path];ok{
   tracks[i].Title=info.Title;tracks[i].Artist=info.Artist;tracks[i].Album=info.Album
  }
 }
 return nil
}
func SaveMetadata(station,path string,details Metadata)error{
 if station==""||path==""||strings.TrimSpace(details.Title)==""||len(details.Title)>300||len(details.Artist)>300||len(details.Album)>300{return errors.New("invalid track metadata")}
 metadataMu.Lock();defer metadataMu.Unlock()
 all,err:=readMetadata();if err!=nil{return err}
 if all[station]==nil{all[station]=map[string]Metadata{}}
 all[station][path]=details
 filename:=metadataPath()
 if err:=os.MkdirAll(filepath.Dir(filename),0700);err!=nil{return err}
 tmp,err:=os.CreateTemp(filepath.Dir(filename),".radio-meta-*");if err!=nil{return err}
 defer os.Remove(tmp.Name())
 if err:=tmp.Chmod(0600);err!=nil{tmp.Close();return err}
 if err:=json.NewEncoder(tmp).Encode(all);err!=nil{tmp.Close();return err}
 if err:=tmp.Sync();err!=nil{tmp.Close();return err}
 if err:=tmp.Close();err!=nil{return err}
 return os.Rename(tmp.Name(),filename)
}
