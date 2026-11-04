package vrchathls

import (
 "bytes"
 "image"
 "image/color"
 "image/png"
 "net/http"
 "net/http/httptest"
 "os"
 "path/filepath"
 "testing"
 "time"
)

func testPNG(t *testing.T,r,g uint8)[]byte{
 t.Helper()
 img:=image.NewRGBA(image.Rect(0,0,4,4))
 for y:=0;y<4;y++{for x:=0;x<4;x++{img.SetRGBA(x,y,color.RGBA{R:r,G:g,A:255})}}
 var buf bytes.Buffer
 if err:=png.Encode(&buf,img);err!=nil{t.Fatal(err)}
 return buf.Bytes()
}
func TestCurrentArtFollowsSongAndManualChanges(t *testing.T){
 isolatedAssets(t)
 const station="neffex"
 path,err:=CurrentArtPath(station);if err!=nil{t.Fatal(err)}
 d:=filepath.Dir(path)
 if err:=os.MkdirAll(d,0750);err!=nil{t.Fatal(err)}
 fallback:=testPNG(t,200,0)
 if err:=os.WriteFile(filepath.Join(d,"cover.png"),fallback,0640);err!=nil{t.Fatal(err)}
 source,stamp,err:=UpdateCurrentArt(station,"THIRST","MAYHEM","",time.Time{})
 if err!=nil{t.Fatal(err)}
 data,err:=os.ReadFile(path);if err!=nil{t.Fatal(err)}
 if !bytes.Equal(data,fallback){t.Fatal("fallback artwork not selected")}
 before,err:=os.Stat(path);if err!=nil{t.Fatal(err)}
 // Repeated HLS frames must not rewrite art.png if nothing changed.
 source,stamp,err=UpdateCurrentArt(station,"THIRST","MAYHEM",source,stamp)
 if err!=nil{t.Fatal(err)}
 after,err:=os.Stat(path);if err!=nil{t.Fatal(err)}
 if !after.ModTime().Equal(before.ModTime()){t.Fatal("unchanged artwork was rewritten")}
 special:=testPNG(t,0,200)
 trackPath,err:=artworkFile(station,"THIRST","MAYHEM");if err!=nil{t.Fatal(err)}
 if err:=os.WriteFile(trackPath,special,0640);err!=nil{t.Fatal(err)}
 source,stamp,err=UpdateCurrentArt(station,"THIRST","MAYHEM",source,stamp)
 if err!=nil{t.Fatal(err)}
 data,err=os.ReadFile(path);if err!=nil{t.Fatal(err)}
 if !bytes.Equal(data,special){t.Fatal("song cover did not replace station fallback")}
 source,stamp,err=UpdateCurrentArt(station,"Different Artist","Next Song",source,stamp)
 if err!=nil{t.Fatal(err)}
 data,err=os.ReadFile(path);if err!=nil{t.Fatal(err)}
 if !bytes.Equal(data,fallback){t.Fatal("next song did not restore fallback")}
 if err:=os.Remove(filepath.Join(d,"cover.png"));err!=nil{t.Fatal(err)}
 _,_,err=UpdateCurrentArt(station,"Different Artist","Next Song",source,stamp)
 if err!=nil{t.Fatal(err)}
 if _,err:=os.Stat(path);!os.IsNotExist(err){t.Fatalf("stale art.png retained without cover: %v",err)}
}
func TestCurrentArtHTTPAndPathSafety(t *testing.T){
 isolatedAssets(t)
 if _,err:=CurrentArtPath("../private");err==nil{t.Fatal("path traversal accepted")}
 rec:=httptest.NewRecorder()
 ServeCurrentArt(rec,httptest.NewRequest(http.MethodGet,"/neffex/art.png",nil),"neffex")
 if rec.Code!=http.StatusNotFound{t.Fatalf("missing artwork status=%d",rec.Code)}
 if rec.Header().Get("Cache-Control")!="no-store"{t.Fatal("current artwork should not be cached by proxy")}
 path,err:=CurrentArtPath("neffex");if err!=nil{t.Fatal(err)}
 if err:=os.MkdirAll(filepath.Dir(path),0750);err!=nil{t.Fatal(err)}
 if err:=os.WriteFile(path,testPNG(t,120,120),0640);err!=nil{t.Fatal(err)}
 rec=httptest.NewRecorder()
 ServeCurrentArt(rec,httptest.NewRequest(http.MethodGet,"/neffex/art.png",nil),"neffex")
 if rec.Code!=http.StatusOK||rec.Header().Get("Content-Type")!="image/png"{t.Fatalf("art response %d %s",rec.Code,rec.Header().Get("Content-Type"))}
}
