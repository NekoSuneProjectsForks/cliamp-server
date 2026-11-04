// Package relay publishes a local cliamp stream to external Icecast servers.
//
// Each configured destination has its own supervised FFmpeg process. A failed
// connection never interrupts the local station or another destination.
package relay

import (
 "context"
 "fmt"
 "log/slog"
 "net"
 "net/url"
 "os"
 "os/exec"
 "strconv"
 "strings"
 "time"
)

type Destination struct {
 Name string
 Host string
 Port int
 Mount string
 PasswordEnv string
 Public bool
 Genre string
 Description string
 TLS bool
}

const retryDelay = 5 * time.Second

// Run keeps a source connected until ctx is cancelled. sourceURL is a
// loopback cliamp MP3 stream, NOT an arbitrary user-supplied external URL.
func Run(ctx context.Context, station, sourceURL string, dst Destination) {
 for ctx.Err() == nil {
  if err := runOnce(ctx, sourceURL, dst); err != nil && ctx.Err() == nil {
   slog.Warn("Icecast relay disconnected", "station", station, "destination", dst.Name, "error", err)
  }
  timer := time.NewTimer(retryDelay)
  select {
  case <-ctx.Done(): timer.Stop(); return
  case <-timer.C:
  }
 }
}

func runOnce(ctx context.Context, sourceURL string, d Destination) error {
 password := os.Getenv(d.PasswordEnv)
 if password == "" { return fmt.Errorf("missing source password environment variable %q", d.PasswordEnv) }
 scheme := "icecast"
 if d.TLS { scheme = "icecasts" }
 host := net.JoinHostPort(d.Host, strconv.Itoa(d.Port))
 mount := "/" + strings.TrimLeft(d.Mount, "/")
 target := (&url.URL{Scheme:scheme, User:url.UserPassword("source",password), Host:host,Path:mount}).String()
 public := "0"
 if d.Public { public="1" }
 args := []string{"-hide_banner","-loglevel","quiet","-nostdin","-reconnect","1","-reconnect_streamed","1",
   "-i",sourceURL,"-map","0:a:0","-vn","-c:a","copy","-content_type","audio/mpeg",
   "-ice_name",d.Name,"-ice_genre",d.Genre,"-ice_description",d.Description,
   "-ice_public",public,"-f","mp3",target}
 cmd := exec.CommandContext(ctx,"ffmpeg",args...)
 // Do not log exec arguments or stderr; both may contain stream credentials.
 if err := cmd.Run(); err != nil && ctx.Err() == nil {
  return fmt.Errorf("FFmpeg publisher exited: %w",err)
 }
 return ctx.Err()
}
