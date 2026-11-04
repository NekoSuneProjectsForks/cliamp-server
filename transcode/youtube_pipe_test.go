package transcode

import (
 "context"
 "io"
 "os"
 "path/filepath"
 "runtime"
 "strings"
 "testing"
 "time"
)

func TestYouTubeUsesExtractorMediaPipeNotSignedURL(t *testing.T){
 if runtime.GOOS=="windows"{t.Skip("test uses POSIX shell fixture")}
 dir:=t.TempDir()
 yt:=`#!/bin/sh
for arg in "$@"; do
 case "$arg" in
  --get-url) echo "must not expose signed media URL" >&2; exit 27;;
 esac
done
printf 'WEBM_BYTES_FROM_EXTRACTOR'
`
 ff:=`#!/bin/sh
for arg in "$@"; do
 case "$arg" in
  https://*|http://*) echo "ffmpeg must not fetch signed URL" >&2; exit 30;;
 esac
done
cat
`
 for name,body:=range map[string]string{"yt-dlp":yt,"ffmpeg":ff}{
  if err:=os.WriteFile(filepath.Join(dir,name),[]byte(body),0755);err!=nil{t.Fatal(err)}
 }
 t.Setenv("PATH",dir+string(os.PathListSeparator)+os.Getenv("PATH"))
 ctx,cancel:=context.WithTimeout(context.Background(),5*time.Second)
 defer cancel()
 reader,err:=NewPCMReader(ctx,"youtube:https://www.youtube.com/watch?v=AMmBY2D9IJU")
 if err!=nil{t.Fatal(err)}
 data,err:=io.ReadAll(reader)
 if err!=nil{t.Fatal(err)}
 if string(data)!="WEBM_BYTES_FROM_EXTRACTOR"{t.Fatalf("wrong pipe bytes %q",string(data))}
 if err:=reader.Close();err!=nil{t.Fatalf("failed to reap processes: %v",err)}
}

func TestUnsupportedYouTubeMediaURLRejectedWithoutNetwork(t *testing.T){
 _,err:=NewPCMReader(context.Background(),"youtube:https://evil.example/watch?v=AMmBY2D9IJU")
 if err==nil||!strings.Contains(err.Error(),"unsupported"){t.Fatalf("expected URL validation error: %v",err)}
}
