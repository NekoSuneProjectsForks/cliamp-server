package vrchathls

import (
 "context"
 "fmt"
 "image"
 "image/color"
 "image/draw"
 "math"
 "os/exec"
 "path/filepath"
 "strings"
 "time"
 "unicode/utf8"
)

const (
 titleRasterWidth = 2048
 titleRasterHeight = 48
 titleBaseFontSize = 26
 titleMaxWidth = Width - 24
 titleMaxHeight = 27
 titleMinScale = 10.0 / titleBaseFontSize
)
const titleFont = "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf"

// rasterTitle uses FFmpeg drawtext ONCE per song/override change.
// At the normal 1 FPS frame rate, the cached overlay is merely composited
// onto the existing Go background frame; no per-frame FFmpeg subprocess.
func rasterTitle(ctx context.Context,textPath string)(*image.RGBA,error){
 renderCtx,cancel:=context.WithTimeout(ctx,4*time.Second)
 defer cancel()
 filter:="drawtext=fontfile="+titleFont+":textfile="+textPath+":expansion=none:fontcolor=white:fontsize=26:x=1:y=1"
 cmd:=exec.CommandContext(renderCtx,"ffmpeg",
  "-hide_banner","-loglevel","error","-nostdin",
  "-f","lavfi","-i","color=c=black@0.0:s=2048x48:r=1,format=rgba",
  "-vf",filter,"-frames:v","1","-f","rawvideo","-pix_fmt","rgba","pipe:1")
 raw,err:=cmd.Output()
 if err!=nil{return nil,fmt.Errorf("render title overlay: %w",err)}
 if len(raw)!=titleRasterWidth*titleRasterHeight*4{return nil,fmt.Errorf("unexpected title raster size %d",len(raw))}
 return &image.RGBA{Pix:raw,Stride:titleRasterWidth*4,Rect:image.Rect(0,0,titleRasterWidth,titleRasterHeight)},nil
}
// glyphBounds inspects real FFmpeg-rendered alpha pixels, not rune count.
// This works for proportional fonts, punctuation and Unicode.
func glyphBounds(src *image.RGBA)image.Rectangle{
 minX,minY,maxX,maxY:=src.Bounds().Max.X,src.Bounds().Max.Y,0,0
 for y:=0;y<src.Bounds().Dy();y++{
  for x:=0;x<src.Bounds().Dx();x++{
   if src.Pix[y*src.Stride+x*4+3]==0{continue}
   if x<minX{minX=x};if x>=maxX{maxX=x+1}
   if y<minY{minY=y};if y>=maxY{maxY=y+1}
  }
 }
 if maxX<=minX||maxY<=minY{return image.Rectangle{}}
 return image.Rect(minX,minY,maxX,maxY)
}
func fittedTitle(ctx context.Context,fullText,file string)(*image.RGBA,error){
 // No input markup reaches FFmpeg's filter parser; it reads the textfile.
 // Clamp pathological metadata and use an ellipsis only below 10px.
 runes:=[]rune(strings.TrimSpace(fullText))
 if len(runes)>140{runes=append(runes[:139],'…')}
 text:=string(runes)
 if text==""{return nil,nil}
 for try:=0;try<3;try++{
  if err:=atomicWrite(file,[]byte(text),0640);err!=nil{return nil,err}
  src,err:=rasterTitle(ctx,file);if err!=nil{return nil,err}
  bounds:=glyphBounds(src)
  if bounds.Empty(){return nil,nil}
  maxRawWidth:=int(math.Round(float64(titleMaxWidth)/titleMinScale))
  // The raster fits all visible characters only when maxX stays safely
  // inside the 2048px source image. Reduce and rerender if necessary.
  if bounds.Dx()>maxRawWidth||bounds.Max.X>=titleRasterWidth-5{
   chars:=[]rune(text)
   if len(chars)>8{
    ratio:=float64(maxRawWidth)/float64(bounds.Dx())
    limit:=int(float64(len(chars))*ratio*0.90)
    if limit>len(chars)-1{limit=len(chars)-1}
    if limit<7{limit=7}
    text=string(chars[:limit])+"…"
    continue
   }
  }
  targetWidth:=bounds.Dx()
  scale:=math.Min(1,math.Min(float64(titleMaxWidth)/float64(bounds.Dx()),float64(titleMaxHeight)/float64(bounds.Dy())))
  if scale<0.01{scale=0.01}
  targetWidth=int(math.Round(float64(bounds.Dx())*scale))
  targetHeight:=int(math.Round(float64(bounds.Dy())*scale))
  if targetWidth<1{targetWidth=1};if targetHeight<1{targetHeight=1}
  out:=image.NewRGBA(image.Rect(0,0,targetWidth,targetHeight))
  // Nearest-neighbour is cheap at one render per title. The text is
  // antialiased by FFmpeg's FreeType rasteriser before scaling.
  for y:=0;y<targetHeight;y++{for x:=0;x<targetWidth;x++{
   sx:=bounds.Min.X+int(float64(x)*float64(bounds.Dx())/float64(targetWidth))
   sy:=bounds.Min.Y+int(float64(y)*float64(bounds.Dy())/float64(targetHeight))
   out.SetRGBA(x,y,src.RGBAAt(sx,sy))
  }}
  return out,nil
 }
 return nil,fmt.Errorf("could not fit long title")
}
func paintTitle(dst *image.RGBA,title *image.RGBA){
 if title==nil{return}
 x:=(Width-title.Bounds().Dx())/2
 y:=140+(34-title.Bounds().Dy())/2
 draw.Draw(dst,image.Rect(x,y,x+title.Bounds().Dx(),y+title.Bounds().Dy()),title,image.Point{},draw.Over)
}
var _=color.RGBA{}
var _=filepath.Join
var _=time.Second
var _=utf8.RuneCountInString
