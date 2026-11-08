package vrchathls

import (
 "context"
 "bytes"
 "crypto/sha256"
 "encoding/hex"
 "encoding/json"
 "fmt"
 "errors"
 "image"
 _ "image/jpeg"
 "image/png"
 "io"
 "mime/multipart"
 "os"
 "os/exec"
 "path/filepath"
 "strings"
 "time"
)

var AssetsDirectory = "/data/vrchat-assets"
const Width = 320
const Height = 180
const FrameSize = Width*Height*3

// Settings are persisted per station and can be safely changed during a live broadcast.
type Settings struct {
 Mode string `json:"mode"` // solid, image, video
 ShowCover bool `json:"show_cover"`
 ShowTitle bool `json:"show_title"`
 TitleOverride string `json:"title_override,omitempty"`
 ArtistOverride string `json:"artist_override,omitempty"`
}
func Defaults() Settings{return Settings{Mode:"solid",ShowCover:true,ShowTitle:true}}
func assetDir(id string) (string,error) {
 if !validID(id){return "",errors.New("invalid station ID")}
 return filepath.Join(AssetsDirectory,id),nil
}
func settingsPath(id string)(string,error){d,err:=assetDir(id);return filepath.Join(d,"settings.json"),err}
func LoadSettings(id string)(Settings,error){
 path,err:=settingsPath(id);if err!=nil{return Settings{},err}
 s:=Defaults()
 b,err:=os.ReadFile(path);if os.IsNotExist(err){return s,nil};if err!=nil{return s,err}
 if err:=json.Unmarshal(b,&s);err!=nil{return Defaults(),err}
 return s,nil
}
func SaveSettings(id string,s Settings) error{
 if s.Mode!="solid"&&s.Mode!="image"&&s.Mode!="video"{return errors.New("mode must be solid, image or video")}
 if len(s.TitleOverride)>160||len(s.ArtistOverride)>160{return errors.New("text override too long")}
 d,err:=assetDir(id);if err!=nil{return err}
 if err:=os.MkdirAll(d,0750);err!=nil{return err}
 b,err:=json.MarshalIndent(s,"","  ");if err!=nil{return err}
 return atomicWrite(filepath.Join(d,"settings.json"),b,0600)
}
func atomicWrite(path string,b []byte,perm os.FileMode)error{
 tmp,err:=os.CreateTemp(filepath.Dir(path),".visual-*");if err!=nil{return err}
 defer os.Remove(tmp.Name())
 if err:=tmp.Chmod(perm);err!=nil{tmp.Close();return err}
 if _,err:=tmp.Write(b);err!=nil{tmp.Close();return err}
 if err:=tmp.Sync();err!=nil{tmp.Close();return err}
 if err:=tmp.Close();err!=nil{return err}
 return os.Rename(tmp.Name(),path)
}
func artworkFile(id,artist,title string)(string,error){
 d,err:=assetDir(id);if err!=nil{return "",err}
 h:=sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(artist))+"\x00"+strings.ToLower(strings.TrimSpace(title))))
 return filepath.Join(d,"track-"+hex.EncodeToString(h[:12])+".png"),nil
}
func sanitizeFileImage(reader io.Reader) ([]byte,error) {
 data,err:=io.ReadAll(io.LimitReader(reader,8<<20+1));if err!=nil{return nil,err}
 if len(data)>8<<20{return nil,errors.New("image exceeds 8 MiB")}
 cfg,_,err:=image.DecodeConfig(bytes.NewReader(data));if err!=nil{return nil,errors.New("unsupported image; upload PNG or JPEG")}
 if cfg.Width<1||cfg.Height<1||cfg.Width>4096||cfg.Height>4096{return nil,errors.New("image dimensions must be 1-4096 pixels")}
 pic,_,err:=image.Decode(bytes.NewReader(data));if err!=nil{return nil,errors.New("invalid image data")}
 // Re-encode to PNG so uploaded files never depend on the supplied extension.
 var output bytes.Buffer
 if err:=png.Encode(&output,pic);err!=nil{return nil,err}
 return output.Bytes(),nil
}
// UploadImage stores a station background, station cover, or per-track cover.
func UploadImage(id,kind,artist,title string,file multipart.File)error{
 d,err:=assetDir(id);if err!=nil{return err}
 if err:=os.MkdirAll(d,0750);err!=nil{return err}
 path:=""
 switch kind{
 case "background": path=filepath.Join(d,"background.png")
 case "cover":path=filepath.Join(d,"cover.png")
 case "track-cover":
  if strings.TrimSpace(title)==""{return errors.New("track title required")}
  path,err=artworkFile(id,artist,title);if err!=nil{return err}
 default:return errors.New("invalid image kind")
 }
 b,err:=sanitizeFileImage(file);if err!=nil{return err}
 return atomicWrite(path,b,0640)
}
// UploadVideo stores only local MP4 data and verifies FFmpeg can decode a video stream.
func UploadVideo(ctx context.Context,id string,file multipart.File)error{
 d,err:=assetDir(id);if err!=nil{return err}
 if err:=os.MkdirAll(d,0750);err!=nil{return err}
 tmp,err:=os.CreateTemp(d,".video-*.mp4");if err!=nil{return err}
 defer os.Remove(tmp.Name())
 n,err:=io.Copy(tmp,io.LimitReader(file,32<<20+1));closeErr:=tmp.Close()
 if err!=nil{return err};if closeErr!=nil{return closeErr}
 if n>32<<20{return errors.New("MP4 exceeds 32 MiB")}
 probeCtx,cancel:=context.WithTimeout(ctx,8*time.Second);defer cancel()
 out,err:=exec.CommandContext(probeCtx,"ffprobe","-v","error","-select_streams","v:0","-show_entries","stream=codec_name","-of","default=nw=1:nk=1",tmp.Name()).Output()
 if err!=nil||strings.TrimSpace(string(out))==""{return errors.New("upload must contain a decodable video stream")}
 // Normalize uploads ONCE at 320x180/1fps. This avoids decoding
 // high-resolution 30/60 FPS footage continuously on a Raspberry Pi.
 normalized,err:=os.CreateTemp(d,".normalized-*.mp4")
 if err!=nil{return err}
 normalizedPath:=normalized.Name()
 normalized.Close()
 defer os.Remove(normalizedPath)
 normalizeCtx,cancelNormalize:=context.WithTimeout(ctx,90*time.Second)
 defer cancelNormalize()
 cmd:=exec.CommandContext(normalizeCtx,"ffmpeg","-hide_banner","-loglevel","error","-nostdin","-y",
  "-i",tmp.Name(),"-t","90","-vf","fps=1,scale=320:180:force_original_aspect_ratio=increase,crop=320:180",
  "-an","-c:v","libx264","-preset","ultrafast","-threads","1","-pix_fmt","yuv420p",
  "-r","1","-g","4","-movflags","+faststart",normalizedPath)
 if err:=cmd.Run();err!=nil{return errors.New("could not convert MP4 into low-power 320x180 background")}
 if st,err:=os.Stat(normalizedPath);err!=nil||st.Size()<100{return errors.New("normalized video missing or empty")}
 return os.Rename(normalizedPath,filepath.Join(d,"background.mp4"))
}
func RemoveAsset(id,kind,artist,title string)error{
 d,err:=assetDir(id);if err!=nil{return err}
 path:=""
 switch kind{
 case "background":path=filepath.Join(d,"background.png")
 case "video":path=filepath.Join(d,"background.mp4")
 case "cover":path=filepath.Join(d,"cover.png")
 case "track-cover":path,err=artworkFile(id,artist,title);if err!=nil{return err}
 default:return errors.New("invalid asset kind")
 }
 if err:=os.Remove(path);err!=nil&&!os.IsNotExist(err){return err}
 return nil
}
// AssetState returns safe UI metadata; no local paths are exposed.
func AssetState(id,artist,title string)map[string]any{
 d,err:=assetDir(id);if err!=nil{return map[string]any{}}
 has:=func(p string)bool{st,e:=os.Stat(p);return e==nil&&st.Mode().IsRegular()}
 track,_:=artworkFile(id,artist,title)
 cfg,_:=LoadSettings(id)
 return map[string]any{"settings":cfg,"background_image":has(filepath.Join(d,"background.png")),"background_video":has(filepath.Join(d,"background.mp4")),"cover":has(filepath.Join(d,"cover.png")),"track_cover":has(track),"current_art":has(filepath.Join(d,"art.png"))}
}

// SaveAutoTrackCover stores a provider-sourced cover only when no manual or
// existing cover exists for this artist/title. Changes appear next frame.
func SaveAutoTrackCover(id,artist,title string,encoded []byte) error {
 if strings.TrimSpace(title)==""{return errors.New("missing title")}
 path,err:=artworkFile(id,artist,title);if err!=nil{return err}
 if err:=os.MkdirAll(filepath.Dir(path),0750);err!=nil{return err}
 data,err:=sanitizeFileImage(bytes.NewReader(encoded));if err!=nil{return err}
 file,err:=os.OpenFile(path,os.O_WRONLY|os.O_CREATE|os.O_EXCL,0640)
 if errors.Is(err,os.ErrExist){return nil}
 if err!=nil{return err}
 _,writeErr:=file.Write(data)
 closeErr:=file.Close()
 if writeErr!=nil||closeErr!=nil{_ = os.Remove(path);return errors.Join(writeErr,closeErr)}
 return nil
}

func AutoTrackCoverPath(id,artist,title string)(string,error){
 d,err:=assetDir(id);if err!=nil{return "",err}
 hash:=sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(artist))+"\\x00"+strings.ToLower(strings.TrimSpace(title))))
 return filepath.Join(d,"auto-track-"+hex.EncodeToString(hash[:12])+".png"),nil
}
func HasAutoTrackCover(id,artist,title string)bool{
 path,err:=AutoTrackCoverPath(id,artist,title);if err!=nil{return false}
 stat,err:=os.Stat(path)
 return err==nil&&stat.Mode().IsRegular()&&stat.Size()>0
}
// SaveCachedTrackCover replaces ONLY the managed auto-cover file. Manually
// uploaded track-*.png files are never overwritten during an admin rescan.
func SaveCachedTrackCover(id,artist,title string,encoded []byte)error{
 if strings.TrimSpace(title)==""{return errors.New("missing artwork title")}
 path,err:=AutoTrackCoverPath(id,artist,title);if err!=nil{return err}
 image,err:=sanitizeFileImage(bytes.NewReader(encoded));if err!=nil{return fmt.Errorf("invalid downloaded artwork: %w",err)}
 if err:=os.MkdirAll(filepath.Dir(path),0750);err!=nil{return err}
 return atomicWrite(path,image,0640)
}
