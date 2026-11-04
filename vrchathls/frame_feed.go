package vrchathls

import (
 "context"
 "errors"
 "image"
 "image/color"
 "image/draw"
 _ "image/jpeg"
 _ "image/png"
 "io"
 "log/slog"
 "os"
 "os/exec"
 "path/filepath"
 "strings"
 "sync"
 "time"
)

// Track returns just metadata, avoiding any dependency on the radio server.
type Track struct{Title,Artist string}

// FrameFeed writes a 320x180 RGB frame at 1 FPS into the SAME long-running
// HLS encoder. Changing artwork or text never stops its audio input.
func FrameFeed(ctx context.Context, stationID string, dst io.WriteCloser, current func()Track) {
 defer dst.Close()
 ticker:=time.NewTicker(time.Second)
 defer ticker.Stop()
 solid:=image.NewRGBA(image.Rect(0,0,Width,Height))
 draw.Draw(solid,solid.Bounds(),&image.Uniform{C:color.RGBA{R:13,G:23,B:31,A:255}},image.Point{},draw.Src)
 var cachedBackground *image.RGBA
 var cachedCover *image.RGBA
 var bgFile, coverFile string
 var bgStamp, coverStamp time.Time
 var video *videoDecoder
 defer func(){if video!=nil{video.Close()}}()
 var lastVideo []byte
 var titleLast string
 d,err:=assetDir(stationID);if err!=nil{return}
 // Send the first frame immediately, before waiting for the next second.
 tick:=func()bool{
  cfg,err:=LoadSettings(stationID);if err!=nil{cfg=Defaults()}
  track:=current()
  title:=track.Title
  artist:=track.Artist
  if cfg.TitleOverride!=""{title=cfg.TitleOverride}
  if cfg.ArtistOverride!=""{artist=cfg.ArtistOverride}
  // Dynamic text is reloaded by FFmpeg's drawtext filter every frame.
  text:=strings.Trim(strings.TrimSpace(artist+" - "+title)," -")
  if !cfg.ShowTitle{text=""}
  if len([]rune(text))>75{text=string([]rune(text)[:75])+"…"}
  if text!=titleLast {
   if err:=atomicWrite(filepath.Join(d,"nowplaying.txt"),[]byte(text),0640);err!=nil{slog.Warn("cannot update HLS now-playing text","station",stationID,"error",err)}
   titleLast=text
  }
  desiredVideo:=filepath.Join(d,"background.mp4")
  if cfg.Mode=="video"{
   if st,e:=os.Stat(desiredVideo);e==nil&&st.Size()>0 {
    if video==nil||video.path!=desiredVideo||!video.stamp.Equal(st.ModTime()){
     if video!=nil{video.Close()}
     video=startVideoDecoder(ctx,desiredVideo,st.ModTime())
     lastVideo=nil
    }
   }else if video!=nil{video.Close();video=nil;lastVideo=nil}
  }else if video!=nil{video.Close();video=nil;lastVideo=nil}
  bg:=solid
  if cfg.Mode=="image" {
   path:=filepath.Join(d,"background.png")
   if st,e:=os.Stat(path);e==nil{
    if bgFile!=path || !st.ModTime().Equal(bgStamp){
     if loaded,e:=loadAndScale(path,Width,Height);e==nil{cachedBackground=loaded;bgFile=path;bgStamp=st.ModTime()}
    }
    if cachedBackground!=nil{bg=cachedBackground}
   }else{cachedBackground=nil;bgFile=""}
  }
  frame:=image.NewRGBA(image.Rect(0,0,Width,Height))
  if video!=nil {
   // Keep using the most recent frame if the decoder is delayed.
   select {case v,ok:=<-video.frames:if ok{lastVideo=v};default:}
   if len(lastVideo)==FrameSize{
    for y:=0;y<Height;y++{
     for x:=0;x<Width;x++{
      off:=(y*Width+x)*3
      frame.SetRGBA(x,y,color.RGBA{lastVideo[off],lastVideo[off+1],lastVideo[off+2],255})
     }
    }
   }else{draw.Draw(frame,frame.Bounds(),bg,image.Point{},draw.Src)}
  }else{draw.Draw(frame,frame.Bounds(),bg,image.Point{},draw.Src)}
  // Semi-transparent bar improves text legibility without a browser renderer.
  bar:=image.Rect(0,132,Width,Height)
  draw.DrawMask(frame,bar,&image.Uniform{C:color.Black},image.Point{},&image.Uniform{C:color.Alpha{A:175}},image.Point{},draw.Over)
  if cfg.ShowCover {
   path:=filepath.Join(d,"cover.png")
   if special,e:=artworkFile(stationID,track.Artist,track.Title);e==nil{
    if st,e:=os.Stat(special);e==nil&&st.Mode().IsRegular(){path=special}
   }
   if st,e:=os.Stat(path);e==nil{
    if coverFile!=path||!st.ModTime().Equal(coverStamp){
     if loaded,e:=loadAndScale(path,88,88);e==nil{cachedCover=loaded;coverFile=path;coverStamp=st.ModTime()}
    }
    if cachedCover!=nil{draw.Draw(frame,image.Rect(12,29,100,117),cachedCover,image.Point{},draw.Over)}
   }else{cachedCover=nil;coverFile=""}
  }
  buf:=make([]byte,FrameSize)
  for y:=0;y<Height;y++{for x:=0;x<Width;x++{
   p:=frame.RGBAAt(x,y);off:=(y*Width+x)*3
   buf[off],buf[off+1],buf[off+2]=p.R,p.G,p.B
  }}
  n,writeErr:=dst.Write(buf)
  return writeErr==nil && n==len(buf)
 }
 if !tick(){return}
 for {
  select {
  case <-ctx.Done():return
  case <-ticker.C:if !tick(){return}
  }
 }
}
func loadAndScale(path string,w,h int)(*image.RGBA,error){
 file,err:=os.Open(path);if err!=nil{return nil,err};defer file.Close()
 src,_,err:=image.Decode(file);if err!=nil{return nil,err}
 out:=image.NewRGBA(image.Rect(0,0,w,h))
 bounds:=src.Bounds()
 sw,sh:=bounds.Dx(),bounds.Dy()
 if sw<1||sh<1{return nil,errors.New("empty image")}
 for y:=0;y<h;y++{for x:=0;x<w;x++{
  sx:=bounds.Min.X+x*sw/w;sy:=bounds.Min.Y+y*sh/h
  out.Set(x,y,src.At(sx,sy))
 }}
 return out,nil
}
type videoDecoder struct {
 path string
 stamp time.Time
 cancel context.CancelFunc
 frames chan []byte
 done chan struct{}
 once sync.Once
}
func startVideoDecoder(parent context.Context,path string,stamp time.Time)*videoDecoder{
 ctx,cancel:=context.WithCancel(parent)
 d:=&videoDecoder{path:path,stamp:stamp,cancel:cancel,frames:make(chan []byte,1),done:make(chan struct{})}
 go func(){
  defer close(d.done)
  defer close(d.frames)
  cmd:=exec.CommandContext(ctx,"ffmpeg","-hide_banner","-loglevel","error","-nostdin","-re","-stream_loop","-1","-i",path,
   "-an","-vf","fps=1,scale=320:180:force_original_aspect_ratio=increase,crop=320:180",
   "-pix_fmt","rgb24","-f","rawvideo","pipe:1")
  stdout,err:=cmd.StdoutPipe()
  if err!=nil{return}
  if err:=cmd.Start();err!=nil{return}
  for ctx.Err()==nil {
   frame:=make([]byte,FrameSize)
   if _,err:=io.ReadFull(stdout,frame);err!=nil{break}
   select {case d.frames<-frame:default: // drop stale frame, keep decoder moving
    select{case <-d.frames:default:}
    select{case d.frames<-frame:default:}
   }
  }
  _=stdout.Close()
  _=cmd.Wait()
 }()
 return d
}
func(d *videoDecoder)Close(){
 d.once.Do(func(){d.cancel();select{case <-d.done:case <-time.After(2*time.Second):}})
}
