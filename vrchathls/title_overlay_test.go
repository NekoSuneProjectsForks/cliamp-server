package vrchathls

import (
 "context"
 "image"
 "os"
 "os/exec"
 "path/filepath"
 "strings"
 "testing"
)

func TestPixelMeasuredHLSOverlayFitsFrame(t *testing.T) {
 if _,err:=exec.LookPath("ffmpeg");err!=nil{t.Skip("ffmpeg unavailable")}
 if _,err:=os.Stat(titleFont);err!=nil{t.Skip("DejaVu font unavailable")}
 file:=filepath.Join(t.TempDir(),"nowplaying.txt")
 examples:=[]string{
  "Neffex - Don't Wanna Let Myself Down",
  "THIRST - MAYHEM",
  "An Artist With A Long Name - A Much Longer Song Title That Previously Was Clipped",
  strings.Repeat("LongTitle",14),
 }
 for _,name:=range examples{
  sprite,err:=fittedTitle(context.Background(),name,file)
  if err!=nil{t.Fatalf("title %q: %v",name,err)}
  if sprite==nil{t.Fatalf("no sprite for %q",name)}
  bounds:=sprite.Bounds()
  if bounds.Dx()>titleMaxWidth||bounds.Dy()>titleMaxHeight{
   t.Fatalf("rendered title exceeds frame bounds: %q size=%v",name,bounds)
  }
  frame:=image.NewRGBA(image.Rect(0,0,Width,Height))
  paintTitle(frame,sprite)
 }
}
