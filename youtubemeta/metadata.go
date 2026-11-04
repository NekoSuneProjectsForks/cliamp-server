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
 cmd:=exec.CommandContext(ctx,"yt-dlp","--ignore-config","--no-playlist","--skip-download","--no-warnings",
  "--js-runtimes","node",
  "--print","%(title)j",
  "--print","%(artist)j",
  "--print","%(track)j",
  "--print","%(album)j",
  "--print","%(creator)j",
  videoURL)
 output,err:=cmd.Output()
 if err!=nil{
  if ctx.Err()!=nil{return Metadata{},ctx.Err()}
  return Metadata{},fmt.Errorf("yt-dlp metadata lookup failed: %w",err)
 }
 values,err:=parsePrintedFields(output)
 if err!=nil{return Metadata{},err}
 if values[0]=="" {return Metadata{},errors.New("YouTube video title missing")}
 artist:=values[1]
 if artist==""{artist=values[4]}
 m:=Clean(values[0],artist,values[2])
 m.Album=values[3]
 return m,nil
}


// parsePrintedFields handles yt-dlp's five JSON-escaped --print values.
// Returning only five short fields avoids allocating a large formats JSON.
func parsePrintedFields(output []byte)([5]string,error){
 var fields [5]string
 if len(output)>64<<10{return fields,errors.New("YouTube metadata exceeds 64 KiB")}
 rows:=strings.Split(strings.TrimSpace(string(output)),"\n")
 if len(rows)!=5{return fields,fmt.Errorf("yt-dlp returned %d metadata fields, want 5",len(rows))}
 for i,row:=range rows{
  if err:=json.Unmarshal([]byte(strings.TrimSpace(row)),&fields[i]);err!=nil{
   return fields,fmt.Errorf("invalid yt-dlp metadata field %d: %w",i,err)
  }
  if fields[i]=="NA"{fields[i]=""}
 }
 return fields,nil
}
