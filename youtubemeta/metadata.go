// Package youtubemeta resolves YouTube titles without fetching media.
// Metadata is looked up with yt-dlp once and persisted by the radio library.
package youtubemeta

import (
 "context"
 "encoding/json"
 "errors"
 "fmt"
 "os/exec"
 "regexp"
 "strings"
 "time"

 "cliamp-server/config"
)

type Metadata struct {
 Artist string `json:"artist"`
 Title string `json:"title"`
 Album string `json:"album,omitempty"`
 RawTitle string `json:"raw_title"`
}

var (
 trailingSquare = regexp.MustCompile(`\s*\[[^\[\]]*\]\s*$`)
 trailingRound = regexp.MustCompile(`(?i)\s*\((?:official\s+(?:music\s+)?video|official\s+audio|audio|visuali[sz]er|lyrics?|lyric\s+video|music\s+video|4k|hd)\)\s*$`)
 separator = regexp.MustCompile(`\s+[-–—]\s+`)
 whitespace = regexp.MustCompile(`\s+`)
)

// Clean strips promotional suffixes from the END ONLY. It keeps legitimate
// title text, embedded brackets, and artists with punctuation intact.
func Clean(raw, fallbackArtist, fallbackTrack string) Metadata {
 original:=strings.TrimSpace(raw)
 cleaned:=original
 for {
  next:=trailingSquare.ReplaceAllString(cleaned,"")
  next=trailingRound.ReplaceAllString(next,"")
  if next==cleaned{break}
  cleaned=strings.TrimSpace(next)
 }
 cleaned=whitespace.ReplaceAllString(cleaned," ")
 result:=Metadata{RawTitle:original}
 if parts:=separator.Split(cleaned,2);len(parts)==2&&strings.TrimSpace(parts[0])!=""&&strings.TrimSpace(parts[1])!=""{
  result.Artist=strings.TrimSpace(parts[0])
  result.Title=strings.TrimSpace(parts[1])
 } else {
  result.Artist=strings.TrimSpace(fallbackArtist)
  result.Title=cleaned
  if strings.TrimSpace(fallbackTrack)!=""&&cleaned==""{result.Title=strings.TrimSpace(fallbackTrack)}
 }
 if result.Title==""{result.Title=original}
 return result
}

// Lookup uses yt-dlp's small single-video JSON output without downloading
// music, playlists, cookies or thumbnails. The caller controls a deadline.
func Lookup(ctx context.Context, videoURL string) (Metadata,error) {
 if !config.ValidYouTubeURL(videoURL){return Metadata{},errors.New("unsupported YouTube video URL")}
 ctx,cancel:=context.WithTimeout(ctx,15*time.Second)
 defer cancel()
 cmd:=exec.CommandContext(ctx,"yt-dlp","--no-playlist","--skip-download","--no-warnings","--no-call-home","--js-runtimes","node","--dump-single-json",videoURL)
 output,err:=cmd.Output()
 if err!=nil{
  if ctx.Err()!=nil{return Metadata{},ctx.Err()}
  return Metadata{},fmt.Errorf("yt-dlp metadata lookup failed: %w",err)
 }
 if len(output)>2<<20{return Metadata{},errors.New("metadata too large")}
 var data struct {
  Title string `json:"title"`
  Artist string `json:"artist"`
  Track string `json:"track"`
  Album string `json:"album"`
  Creator string `json:"creator"`
 }
 if err:=json.Unmarshal(output,&data);err!=nil{return Metadata{},fmt.Errorf("invalid yt-dlp metadata: %w",err)}
 if strings.TrimSpace(data.Title)==""{return Metadata{},errors.New("YouTube video title missing")}
 artist:=data.Artist
 if artist==""{artist=data.Creator}
 m:=Clean(data.Title,artist,data.Track)
 m.Album=strings.TrimSpace(data.Album)
 return m,nil
}
