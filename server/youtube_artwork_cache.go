package server

import (
 "context"
 "bytes"
 "image"
 _ "image/jpeg"
 _ "image/png"
 "errors"
 "fmt"
 "io"
 "net/http"
 "net/url"
 "os"
 "path/filepath"
 "strings"
 "time"

 "cliamp-server/library"
 "cliamp-server/vrchathls"
)

const sharedYouTubeArtworkDirectory="/data/youtube-artwork"
func sharedArtworkPath(videoURL string)string{
 id:=library.YouTubeVideoID(videoURL)
 if id==""{return ""}
 directory:=sharedYouTubeArtworkDirectory
 if value:=os.Getenv("CLIAMP_YOUTUBE_ARTWORK_DIR");value!=""{directory=value}
 return filepath.Join(directory,id+".png")
}
// reuseCachedCover is local-only: it makes no iTunes or YouTube API call.
func reuseCachedCover(station,videoURL,artist,title string) {
 path:=sharedArtworkPath(videoURL);if path==""{return}
 data,err:=os.ReadFile(path);if err!=nil{return}
 source:=library.ArtworkIdentity(station,"youtube:"+videoURL,artist,title,"")
 if err:=vrchathls.SaveSourceArtwork(station,source.SourceType+":"+source.SourceKey,artist,title,data);err!=nil{
  // Existing manual cover takes priority; failed image files are nonfatal.
  _=err
 }
}
// cacheYouTubeArtwork performs at most one iTunes search plus a single
// trusted mzstatic image request per *new video ID*. A missing match is
// cached too, avoiding repeat calls on every Pi reboot and each song.
func cacheYouTubeArtwork(ctx context.Context,videoURL,artist,title string)(string,error){
 if artist==""||title==""{return "",nil}
 path:=sharedArtworkPath(videoURL);if path==""{return "",fmt.Errorf("invalid YouTube ID")}
 if _,err:=os.Stat(path);err==nil{return "",nil}
 searchCtx,cancel:=context.WithTimeout(ctx,7*time.Second);defer cancel()
 // Existing artwork matcher insists on matching artist/title, not merely
 // the first iTunes search result.
 finder:=newArtworkLookup()
 endpoint:=finder.coverWithContext(searchCtx,artist,title,"")
 if endpoint==""{return "",nil}
 parsed,err:=url.Parse(endpoint)
 if err!=nil||parsed.Scheme!="https"||parsed.Hostname()!="is1-ssl.mzstatic.com"&&parsed.Hostname()!="is2-ssl.mzstatic.com"&&parsed.Hostname()!="is3-ssl.mzstatic.com"&&parsed.Hostname()!="is4-ssl.mzstatic.com"&&parsed.Hostname()!="is5-ssl.mzstatic.com" {
  // Strict allowlist; no arbitrary server-side image requests.
  return "",fmt.Errorf("artwork host not allowed")
 }
 request,err:=http.NewRequestWithContext(searchCtx,http.MethodGet,endpoint,nil)
 if err!=nil{return "",err}
 client:=&http.Client{Timeout:5*time.Second,CheckRedirect:func(*http.Request,[]*http.Request)error{
  // Never follow provider artwork redirects to an arbitrary host.
  return http.ErrUseLastResponse
 }}
 response,err:=client.Do(request);if err!=nil{return "",err}
 defer response.Body.Close()
 if response.StatusCode!=http.StatusOK{return "",fmt.Errorf("cover request HTTP %d",response.StatusCode)}
 if ct:=response.Header.Get("Content-Type");!strings.HasPrefix(ct,"image/"){return "",fmt.Errorf("cover is not an image")}
 data,err:=io.ReadAll(io.LimitReader(response.Body,2<<20+1));if err!=nil{return "",err}
 if len(data)>2<<20{return "",fmt.Errorf("cover image too large")}
 cfg,format,decodeErr:=image.DecodeConfig(bytes.NewReader(data))
 if decodeErr!=nil||(format!="png"&&format!="jpeg")||cfg.Width<1||cfg.Height<1||cfg.Width>3000||cfg.Height>3000 {
  return "",errors.New("cover is not a supported PNG/JPEG image")
 }
 if err:=os.MkdirAll(filepath.Dir(path),0750);err!=nil{return "",err}
 tmp,err:=os.CreateTemp(filepath.Dir(path),".cover-*.png");if err!=nil{return "",err}
 defer os.Remove(tmp.Name())
 if _,err:=tmp.Write(data);err!=nil{tmp.Close();return "",err}
 if err:=tmp.Close();err!=nil{return "",err}
 // SaveAutoTrackCover validates the binary content. The shared file is
 // further validated on import and isn't served directly to the public.
 if err:=os.Rename(tmp.Name(),path);err!=nil{return "",err}
 return endpoint,nil
}
