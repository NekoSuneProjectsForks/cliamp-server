package vrchathls

import (
 "errors"
 "bytes"
 "io"
 "net/http"
 "os"
 "path/filepath"
 "time"
)

// CurrentArtPath is the station's materialized currently-playing cover,
// alongside nowplaying.txt. It updates on song changes, new automatic
// iTunes artwork, manual cover changes, and fallback image replacement.
func CurrentArtPath(station string)(string,error){
 dir,err:=assetDir(station);if err!=nil{return "",err}
 return filepath.Join(dir,"art.png"),nil
}
func selectedCover(station,artist,title string)(string,os.FileInfo,error){
 return selectedCoverForSource(station,"",artist,title)
}
func selectedCoverForSource(station,source,artist,title string)(string,os.FileInfo,error){
 dir,err:=assetDir(station);if err!=nil{return "",nil,err}
 choices:=[]string{}
 if title!=""{
  path,err:=artworkFile(station,artist,title);if err!=nil{return "",nil,err}
  choices=append(choices,path)
  if source!=""{
   sourceArt,err:=sourceCoverPath(station,source,artist,title)
   if err!=nil{return "",nil,err}
   choices=append(choices,sourceArt)
  }
  if source==""{
   // Legacy source-agnostic covers are not safe when two source files have
   // the same artist and title but different legitimate album images.
   auto,err:=AutoTrackCoverPath(station,artist,title);if err!=nil{return "",nil,err}
   choices=append(choices,auto)
  }
 }
 choices=append(choices,filepath.Join(dir,"cover.png"),filepath.Join(dir,"fallback.png"))
 for _,path:=range choices{
  info,err:=os.Stat(path)
  if err==nil&&info.Mode().IsRegular(){return path,info,nil}
  if err!=nil&&!errors.Is(err,os.ErrNotExist){return "",nil,err}
 }
 return "",nil,nil
}

// UpdateCurrentArt performs only local filesystem operations. It never
// queries iTunes/yt-dlp or spawns FFmpeg and is safe to call each second.
// The last source stamp is passed by the frame feeder to avoid redundant
// writes and keep Raspberry Pi disk I/O low.
func UpdateCurrentArt(station,artist,title,lastSource string,lastStamp time.Time)(string,time.Time,error){
 return UpdateCurrentArtForSource(station,"",artist,title,lastSource,lastStamp)
}
// UpdateCurrentArtForSource prevents two tracks with identical artist/title
// but different file or YouTube identities from sharing downloaded cover art.
func UpdateCurrentArtForSource(station,source,artist,title,lastSource string,lastStamp time.Time)(string,time.Time,error){
 sourcePath,info,err:=selectedCoverForSource(station,source,artist,title)
 if err!=nil{return lastSource,lastStamp,err}
 target,err:=CurrentArtPath(station);if err!=nil{return lastSource,lastStamp,err}
 if sourcePath==""{
  if err:=os.Remove(target);err!=nil&&!errors.Is(err,os.ErrNotExist){return lastSource,lastStamp,err}
  return "",time.Time{},nil
 }
 if sourcePath==lastSource&&info.ModTime().Equal(lastStamp){
  if _,err:=os.Stat(target);err==nil{return lastSource,lastStamp,nil}
 }
 if err:=os.MkdirAll(filepath.Dir(target),0750);err!=nil{return lastSource,lastStamp,err}
 src,err:=os.Open(sourcePath);if err!=nil{return lastSource,lastStamp,err}
 defer src.Close()
 tmp,err:=os.CreateTemp(filepath.Dir(target),".art-*.png");if err!=nil{return lastSource,lastStamp,err}
 defer os.Remove(tmp.Name())
 if err:=tmp.Chmod(0640);err!=nil{tmp.Close();return lastSource,lastStamp,err}
 if _,err:=io.Copy(tmp,io.LimitReader(src,8<<20+1));err!=nil{tmp.Close();return lastSource,lastStamp,err}
 if err:=tmp.Close();err!=nil{return lastSource,lastStamp,err}
 if err:=os.Rename(tmp.Name(),target);err!=nil{return lastSource,lastStamp,err}
 return sourcePath,info.ModTime(),nil
}
// ServeCurrentArt exposes only the generated current-song PNG. A 404 is
// returned when no song-specific or fallback cover is available.
func ServeCurrentArt(w http.ResponseWriter,r *http.Request,station string){
 path,err:=CurrentArtPath(station);if err!=nil{http.NotFound(w,r);return}
 w.Header().Set("Cache-Control","no-store")
 w.Header().Set("Content-Type","image/png")
 w.Header().Set("X-Content-Type-Options","nosniff")
 http.ServeFile(w,r,path)
}

// SaveFallbackCover writes the locally generated CD-case art once per station.
// A saved station cover and per-track artwork always take priority over it.
func SaveFallbackCover(station string, pngData []byte)error {
 dir,err:=assetDir(station);if err!=nil{return err}
 if len(pngData)==0{return errors.New("empty fallback cover")}
 path:=filepath.Join(dir,"fallback.png")
 if _,err:=os.Stat(path);err==nil{return nil}else if !errors.Is(err,os.ErrNotExist){return err}
 if err:=os.MkdirAll(dir,0750);err!=nil{return err}
 return atomicWrite(path,pngData,0640)
}

// HasTrackCover permits the metadata worker to reuse saved artwork without
// repeating an external catalogue lookup for previously downloaded songs.
func HasTrackCover(station,artist,title string)bool{
 path,err:=artworkFile(station,artist,title)
 if err!=nil{return false}
 info,err:=os.Stat(path)
 return err==nil&&info.Mode().IsRegular()&&info.Size()>0
}

// ReadCurrentArt only reads local generated artwork. No HTTP requests occur.
func ReadCurrentArt(station string)([]byte,error){
 path,err:=CurrentArtPath(station);if err!=nil{return nil,err}
 data,err:=os.ReadFile(path);if err!=nil{return nil,err}
 if len(data)==0||len(data)>4<<20{return nil,errors.New("invalid artwork size")}
 // These files must be valid PNG, regardless of filename.
 if !bytes.HasPrefix(data,[]byte{137,80,78,71,13,10,26,10}){return nil,errors.New("invalid PNG artwork")}
 return data,nil
}
