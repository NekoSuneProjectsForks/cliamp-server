// Package vrchathls creates optional H.264/AAC HLS streams for VRChat AVPro.
// It consumes the existing per-station MP3 broadcast and never changes the
// original /<station>/stream endpoint.
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

// Run supervises the FFmpeg HLS encoder until the parent server shuts down.
// Every station gets a separate directory and continuous, playable HLS output.
func Run(ctx context.Context, stationID, streamURL string) {
 if !validID(stationID) {slog.Error("invalid VRChat station ID","id",stationID);return}
 directory:=filepath.Join(Directory,stationID)
 if err:=os.MkdirAll(directory,0750);err!=nil{slog.Error("cannot create HLS directory","station",stationID,"error",err);return}
 for ctx.Err()==nil {
  // Old segments from previous encoder generations have no valid continuity.
  entries,err:=os.ReadDir(directory)
  if err==nil {for _,e:=range entries {if !e.IsDir() && (e.Name()=="index.m3u8" || segmentName.MatchString(e.Name()) || strings.HasSuffix(e.Name(),".tmp")) {_=os.Remove(filepath.Join(directory,e.Name()))}}}
  if err:=encode(ctx,streamURL,directory);err!=nil && ctx.Err()==nil{
   slog.Warn("VRChat HLS encoder stopped; retrying","station",stationID,"error",err)
  }
  timer:=time.NewTimer(retryDelay)
  select {case <-ctx.Done():timer.Stop();return;case <-timer.C:}
 }
}

func validID(id string) bool {
 if id=="" || len(id)>64 {return false}
 for _,r:=range id{if !((r>='a'&&r<='z')||(r>='0'&&r<='9')||r=='-'){return false}}
 return true
}

func encode(ctx context.Context,streamURL,directory string) error {
 // A low-FPS, tiny constant image is used because VRChat's AVPro generally
 // handles MPEG-TS H.264/AAC more consistently than audio-only HLS.
 args:=[]string{
  "-hide_banner","-loglevel","warning","-nostdin",
  "-i",streamURL,
  "-f","lavfi","-i","color=c=black:s=320x180:r=1",
  "-map","1:v:0","-map","0:a:0",
  "-c:v","libx264","-preset","ultrafast","-tune","zerolatency",
  "-pix_fmt","yuv420p","-profile:v","baseline","-r","1",
  "-g","2","-keyint_min","2","-sc_threshold","0",
  "-c:a","aac","-b:a","128k","-ar","44100","-ac","2",
  "-max_muxing_queue_size","1024",
  "-f","hls","-hls_time","4","-hls_list_size","6",
  "-hls_flags","delete_segments+omit_endlist+program_date_time+temp_file",
  "-hls_segment_filename",filepath.Join(directory,"segment_%06d.ts"),
  filepath.Join(directory,"index.m3u8"),
 }
 cmd:=exec.CommandContext(ctx,"ffmpeg",args...)
 cmd.Stderr=os.Stderr
 err:=cmd.Run()
 if ctx.Err()!=nil{return ctx.Err()}
 return err
}

// Serve exposes only a station's generated manifest and MPEG-TS segments.
// Media is read from disk without starting a new per-listener transcoder.
func Serve(w http.ResponseWriter,r *http.Request,stationID string) {
 if !validID(stationID){http.NotFound(w,r);return}
 filename:=r.PathValue("file")
 if filename!="index.m3u8" && !segmentName.MatchString(filename){http.NotFound(w,r);return}
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
