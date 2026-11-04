package vrchathls

import (
 "errors"
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
 dir,err:=assetDir(station);if err!=nil{return "",nil,err}
 choices:=[]string{}
 if title!=""{
  path,err:=artworkFile(station,artist,title);if err!=nil{return "",nil,err}
  choices=append(choices,path)
 }
 choices=append(choices,filepath.Join(dir,"cover.png"))
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
 source,info,err:=selectedCover(station,artist,title)
 if err!=nil{return lastSource,lastStamp,err}
 target,err:=CurrentArtPath(station);if err!=nil{return lastSource,lastStamp,err}
 if source==""{
  if err:=os.Remove(target);err!=nil&&!errors.Is(err,os.ErrNotExist){return lastSource,lastStamp,err}
  return "",time.Time{},nil
 }
 if source==lastSource&&info.ModTime().Equal(lastStamp){
  if _,err:=os.Stat(target);err==nil{return lastSource,lastStamp,nil}
 }
 if err:=os.MkdirAll(filepath.Dir(target),0750);err!=nil{return lastSource,lastStamp,err}
 src,err:=os.Open(source);if err!=nil{return lastSource,lastStamp,err}
 defer src.Close()
 tmp,err:=os.CreateTemp(filepath.Dir(target),".art-*.png");if err!=nil{return lastSource,lastStamp,err}
 defer os.Remove(tmp.Name())
 if err:=tmp.Chmod(0640);err!=nil{tmp.Close();return lastSource,lastStamp,err}
 if _,err:=io.Copy(tmp,io.LimitReader(src,8<<20+1));err!=nil{tmp.Close();return lastSource,lastStamp,err}
 if err:=tmp.Close();err!=nil{return lastSource,lastStamp,err}
 if err:=os.Rename(tmp.Name(),target);err!=nil{return lastSource,lastStamp,err}
 return source,info.ModTime(),nil
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
