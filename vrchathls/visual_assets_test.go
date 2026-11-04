package vrchathls

import (
 "bytes"
 "context"
 "image"
 "image/color"
 "image/png"
 "os"
 "path/filepath"
 "strings"
 "testing"
)

func isolatedAssets(t *testing.T) {
 t.Helper()
 previous:=AssetsDirectory
 AssetsDirectory=t.TempDir()
 t.Cleanup(func(){AssetsDirectory=previous})
}
func TestVisualSettingsPersistence(t *testing.T){
 isolatedAssets(t)
 cfg:=Settings{Mode:"image",ShowCover:true,ShowTitle:true,TitleOverride:"Test title"}
 if err:=SaveSettings("neffex",cfg);err!=nil{t.Fatal(err)}
 got,err:=LoadSettings("neffex");if err!=nil{t.Fatal(err)}
 if got!=cfg{t.Fatalf("settings changed: got=%+v want=%+v",got,cfg)}
 cfg.Mode="malicious"
 if err:=SaveSettings("neffex",cfg);err==nil{t.Fatal("invalid mode accepted")}
 if _,err:=LoadSettings("../secret");err==nil{t.Fatal("path traversal accepted")}
}
func TestImageUploadAndAlbumArtwork(t *testing.T){
 isolatedAssets(t)
 img:=image.NewRGBA(image.Rect(0,0,64,64))
 for y:=0;y<64;y++{for x:=0;x<64;x++{img.SetRGBA(x,y,color.RGBA{R:240,A:255})}}
 var raw bytes.Buffer
 if err:=png.Encode(&raw,img);err!=nil{t.Fatal(err)}
 file:=filepath.Join(t.TempDir(),"pic.png")
 if err:=os.WriteFile(file,raw.Bytes(),0600);err!=nil{t.Fatal(err)}
 open:=func()*os.File{f,e:=os.Open(file);if e!=nil{t.Fatal(e)};return f}
 a:=open();if err:=UploadImage("station","background","","",a);err!=nil{t.Fatal(err)};a.Close()
 a=open();if err:=UploadImage("station","track-cover","Artist","Song",a);err!=nil{t.Fatal(err)};a.Close()
 state:=AssetState("station","Artist","Song")
 if state["background_image"]!=true||state["track_cover"]!=true{t.Fatal(state)}
 if state["cover"]!=false{t.Fatal("incorrect cover state")}
 albumPath,err:=artworkFile("station","Artist","Song");if err!=nil{t.Fatal(err)}
 if !strings.HasSuffix(albumPath,".png"){t.Fatal(albumPath)}
 if err:=RemoveAsset("station","track-cover","Artist","Song");err!=nil{t.Fatal(err)}
 if AssetState("station","Artist","Song")["track_cover"]!=false{t.Fatal("track cover not removed")}
}
func TestScaledImageAndFallback(t *testing.T){
 isolatedAssets(t)
 img:=image.NewRGBA(image.Rect(0,0,8,8))
 img.SetRGBA(0,0,color.RGBA{G:220,A:255})
 path:=filepath.Join(t.TempDir(),"image.png")
 f,err:=os.Create(path);if err!=nil{t.Fatal(err)}
 if err:=png.Encode(f,img);err!=nil{t.Fatal(err)};f.Close()
 scaled,err:=loadAndScale(path,320,180);if err!=nil{t.Fatal(err)}
 if scaled.Bounds().Dx()!=320||scaled.Bounds().Dy()!=180{t.Fatal(scaled.Bounds())}
 if scaled.RGBAAt(0,0).G!=220{t.Fatal("image pixels not preserved")}
 if err:=UploadImage("station","unknown","","",bytesToFile(t,path));err==nil{t.Fatal("unexpected invalid kind acceptance")}
}
func bytesToFile(t *testing.T,path string)*os.File {
 t.Helper();f,err:=os.Open(path);if err!=nil{t.Fatal(err)};t.Cleanup(func(){f.Close()});return f
}
func TestVisualDefaultAndVideoID(t *testing.T){
 isolatedAssets(t)
 cfg,err:=LoadSettings("radio");if err!=nil{t.Fatal(err)}
 if cfg.Mode!="solid"||!cfg.ShowCover||!cfg.ShowTitle{t.Fatal(cfg)}
 if validID("../bad")||!validID("neffex-radio"){t.Fatal("invalid HLS ID validation")}
 _=context.Background()
}

func TestNowPlayingTextFitsVideo(t *testing.T){
 long:="https://www.youtube.com/watch?v=z2ExcessivelyLongExample"
 shown:=fitNowPlaying(long,30)
 if len([]rune(shown))>31||shown==long||!strings.HasSuffix(shown,"…"){t.Fatalf("long title not shortened: %q",shown)}
 if short:=fitNowPlaying("THIRST - MAYHEM",30);short!="THIRST - MAYHEM"{t.Fatalf("short title changed: %q",short)}
 if cjk:=fitNowPlaying(strings.Repeat("音",30),30);len([]rune(cjk))>=30{t.Fatalf("wide glyphs not limited: %q",cjk)}
}
func TestAutomaticCoverDoesNotReplaceManualUpload(t *testing.T){
 isolatedAssets(t)
 pic:=image.NewRGBA(image.Rect(0,0,4,4))
 pic.SetRGBA(0,0,color.RGBA{R:255,A:255})
 var first bytes.Buffer
 if err:=png.Encode(&first,pic);err!=nil{t.Fatal(err)}
 if err:=SaveAutoTrackCover("neffex","THIRST","MAYHEM",first.Bytes());err!=nil{t.Fatal(err)}
 path,err:=artworkFile("neffex","THIRST","MAYHEM");if err!=nil{t.Fatal(err)}
 before,err:=os.ReadFile(path);if err!=nil{t.Fatal(err)}
 pic.SetRGBA(0,0,color.RGBA{G:255,A:255})
 var second bytes.Buffer
 if err:=png.Encode(&second,pic);err!=nil{t.Fatal(err)}
 if err:=SaveAutoTrackCover("neffex","THIRST","MAYHEM",second.Bytes());err!=nil{t.Fatal(err)}
 after,err:=os.ReadFile(path);if err!=nil{t.Fatal(err)}
 if !bytes.Equal(before,after){t.Fatal("automatic cover overwrote existing artwork")}
}
