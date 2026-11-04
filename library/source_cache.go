package library

import (
 "encoding/json"
 "errors"
 "net/url"
 "os"
 "path/filepath"
 "strings"
 "sync"
)

// SourceCacheEntry is stored once per canonical YouTube video ID, not once
// per station or watch URL. SoundCloud and other sources are not enabled here.
type SourceCacheEntry struct{
 Title string `json:"title"`
 Artist string `json:"artist"`
 Album string `json:"album,omitempty"`
 OriginalTitle string `json:"original_title"`
 ArtworkChecked bool `json:"artwork_checked"`
 ArtworkURL string `json:"artwork_url,omitempty"`
}
var sourceCacheMu sync.Mutex
func sourceCacheFile()string{
 if p:=os.Getenv("CLIAMP_SOURCE_CACHE_FILE");p!=""{return p}
 return "/data/youtube-source-cache.json"
}
// YouTubeVideoID accepts watch URLs from the supported hosts and short links.
func YouTubeVideoID(raw string)string{
 u,err:=url.Parse(raw);if err!=nil||u.Scheme!="https"||u.User!=nil||u.Port()!=""{return ""}
 var id string
 switch strings.ToLower(u.Hostname()){
 case "youtube.com","www.youtube.com","m.youtube.com","music.youtube.com":
  if u.Path!="/watch"{return ""};id=u.Query().Get("v")
 case "youtu.be","www.youtu.be":
  if u.Path==""||strings.Count(u.Path,"/")!=1{return ""}
  id=strings.TrimPrefix(u.Path,"/")
 default:return ""
 }
 if len(id)!=11{return ""}
 for _,r:=range id {if !(r>='a'&&r<='z'||r>='A'&&r<='Z'||r>='0'&&r<='9'||r=='-'||r=='_'){return ""}}
 return id
}
func readSourceCache()(map[string]SourceCacheEntry,error){
 b,err:=os.ReadFile(sourceCacheFile())
 if errors.Is(err,os.ErrNotExist){return map[string]SourceCacheEntry{},nil}
 if err!=nil{return nil,err}
 var entries map[string]SourceCacheEntry
 if err:=json.Unmarshal(b,&entries);err!=nil{return nil,err}
 if entries==nil{entries=map[string]SourceCacheEntry{}}
 return entries,nil
}
func CachedYouTube(raw string)(SourceCacheEntry,bool,error){
 id:=YouTubeVideoID(raw);if id==""{return SourceCacheEntry{},false,errors.New("invalid YouTube URL")}
 sourceCacheMu.Lock();defer sourceCacheMu.Unlock()
 entries,err:=readSourceCache();if err!=nil{return SourceCacheEntry{},false,err}
 entry,ok:=entries[id];return entry,ok,nil
}
func SaveCachedYouTube(raw string,entry SourceCacheEntry)error{
 id:=YouTubeVideoID(raw);if id==""||strings.TrimSpace(entry.Title)==""{return errors.New("invalid cached YouTube metadata")}
 sourceCacheMu.Lock();defer sourceCacheMu.Unlock()
 entries,err:=readSourceCache();if err!=nil{return err}
 entries[id]=entry
 filename:=sourceCacheFile()
 if err:=os.MkdirAll(filepath.Dir(filename),0700);err!=nil{return err}
 tmp,err:=os.CreateTemp(filepath.Dir(filename),".youtube-source-*");if err!=nil{return err}
 defer os.Remove(tmp.Name())
 if err:=tmp.Chmod(0600);err!=nil{tmp.Close();return err}
 if err:=json.NewEncoder(tmp).Encode(entries);err!=nil{tmp.Close();return err}
 if err:=tmp.Sync();err!=nil{tmp.Close();return err}
 if err:=tmp.Close();err!=nil{return err}
 return os.Rename(tmp.Name(),filename)
}
// ApplySourceCache fills gaps only. Existing manual per-station edits always win.
func ApplySourceCache(station string,tracks []Track)error{
 for i:=range tracks{
  if !strings.HasPrefix(tracks[i].Path,"youtube:"){continue}
  if _,saved,err:=MetadataFor(station,tracks[i].Path);err!=nil{return err}else if saved{continue}
  entry,ok,err:=CachedYouTube(strings.TrimPrefix(tracks[i].Path,"youtube:"))
  if err!=nil{return err}
  if ok&&entry.Title!=""{
   tracks[i].Title=entry.Title;tracks[i].Artist=entry.Artist;tracks[i].Album=entry.Album
  }
 }
 return nil
}
