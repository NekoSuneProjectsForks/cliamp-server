// Package vrchathls serves a shared low-resolution H.264/AAC HLS stream.
// Only FFmpeg encodes video and audio; a small Go frame feeder allows live
// background and cover changes without replacing the HLS encoder.
package vrchathls

import (
 "context"
 "errors"
 "log/slog"
 "net/http"
 "os"
 "os/exec"
 "path/filepath"
 "regexp"
 "strings"
 "time"
)

const Directory = "/data/vrchat-hls"
const retryDelay = 3*time.Second
var segmentName = regexp.MustCompile(`^segment_[0-9]+\.ts$`)

// Run starts one publisher for a station. The original MP3 radio keeps
// broadcasting irrespective of this visual output.
func Run(ctx context.Context, stationID, streamURL string, current func()Track, online func()bool) {
 if !validID(stationID){slog.Error("invalid VRChat station ID","id",stationID);return}
 dir:=filepath.Join(Directory,stationID)
 asset,err:=assetDir(stationID);if err!=nil{return}
 if err:=os.MkdirAll(dir,0750);err!=nil{slog.Error("cannot create HLS directory","station",stationID,"error",err);return}
 if err:=os.MkdirAll(asset,0750);err!=nil{slog.Error("cannot create visual directory","station",stationID,"error",err);return}
 for ctx.Err()==nil {
  if !online() {
   select{case <-ctx.Done():return;case <-time.After(time.Second):continue}
  }
  // Only remove prior generation files when restarting this publisher.
  entries,err:=os.ReadDir(dir)
  if err==nil{for _,e:=range entries{
   if !e.IsDir() && (e.Name()=="index.m3u8"||segmentName.MatchString(e.Name())||strings.HasSuffix(e.Name(),".tmp")){_ = os.Remove(filepath.Join(dir,e.Name()))}
  }}
  if err:=atomicWrite(filepath.Join(asset,"nowplaying.txt"),[]byte("Live radio"),0640);err!=nil{slog.Warn("cannot prepare HLS metadata","station",stationID,"error",err)}
  if err:=encode(ctx,streamURL,dir,asset,stationID,current,online);err!=nil&&ctx.Err()==nil{
   slog.Warn("VRChat HLS encoder stopped","station",stationID,"error",err)
  }
  select{case <-ctx.Done():return;case <-time.After(retryDelay):}
 }
}
func validID(id string)bool{
 if id==""||len(id)>64{return false}
 for _,r:=range id{if !((r>='a'&&r<='z')||(r>='0'&&r<='9')||r=='-'){return false}}
 return true
}
func encode(ctx context.Context,streamURL,dir,asset,id string,current func()Track,online func()bool)error{
 encoderCtx,cancel:=context.WithCancel(ctx);defer cancel()
 textfile:=filepath.Join(asset,"nowplaying.txt")
 args:=[]string{
  "-hide_banner","-loglevel","warning","-nostdin",
  "-thread_queue_size","8","-f","rawvideo","-pixel_format","rgb24","-video_size","320x180","-framerate","1","-i","pipe:0",
  "-thread_queue_size","128","-i",streamURL,
  "-filter:v","drawtext=textfile="+textfile+":reload=1:expansion=none:fontcolor=white:fontsize=16:x=12:y=148",
  "-map","0:v:0","-map","1:a:0",
  "-c:v","libx264","-preset","ultrafast","-tune","zerolatency","-threads","2",
  "-pix_fmt","yuv420p","-profile:v","baseline","-r","1","-g","4","-keyint_min","4","-sc_threshold","0",
  "-c:a","aac","-b:a","96k","-ar","44100","-ac","2",
  "-max_muxing_queue_size","128",
  "-f","hls","-hls_time","4","-hls_list_size","6",
  "-hls_flags","delete_segments+omit_endlist+program_date_time+temp_file",
  "-hls_segment_filename",filepath.Join(dir,"segment_%06d.ts"),
  filepath.Join(dir,"index.m3u8"),
 }
 cmd:=exec.CommandContext(encoderCtx,"ffmpeg",args...)
 cmd.Stderr=os.Stderr
 stdin,err:=cmd.StdinPipe();if err!=nil{return err}
 if err:=cmd.Start();err!=nil{stdin.Close();return err}
 feedDone:=make(chan struct{})
 go func(){defer close(feedDone);FrameFeed(encoderCtx,id,stdin,current)}()
 done:=make(chan error,1)
 go func(){done<-cmd.Wait()}()
 watch:=time.NewTicker(time.Second);defer watch.Stop()
 for{
  select{
  case <-ctx.Done():cancel();<-done;<-feedDone;return ctx.Err()
  case err:=<-done:cancel();<-feedDone;return err
  case <-watch.C:if !online(){cancel();<-done;<-feedDone;return nil}
  }
 }
}

// Serve returns only the manifest or sanitized TS segment names.
func Serve(w http.ResponseWriter,r *http.Request,stationID string){
 if !validID(stationID){http.NotFound(w,r);return}
 filename:=r.PathValue("file")
 if filename!="index.m3u8"&&!segmentName.MatchString(filename){http.NotFound(w,r);return}
 path:=filepath.Join(Directory,stationID,filename)
 if filename=="index.m3u8"{
  w.Header().Set("Content-Type","application/vnd.apple.mpegurl")
  w.Header().Set("Cache-Control","no-store")
 }else{
  w.Header().Set("Content-Type","video/mp2t")
  w.Header().Set("Cache-Control","public, max-age=12")
 }
 w.Header().Set("X-Content-Type-Options","nosniff")
 if _,err:=os.Stat(path);err!=nil{
  if errors.Is(err,os.ErrNotExist){http.Error(w,"HLS station warming up",http.StatusServiceUnavailable);return}
  http.Error(w,"HLS media unavailable",http.StatusInternalServerError);return
 }
 http.ServeFile(w,r,path)
}
