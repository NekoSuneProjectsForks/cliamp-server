package handler

import (
 "bytes"
 "io"
 "net/http"
 "net/http/httptest"
 "strconv"
 "strings"
 "testing"
 "time"

 "cliamp-server/broadcast"
)

func TestYouTubeMetadataDeliveredViaICYWithoutSourceURL(t *testing.T){
 hub:=broadcast.NewHub("radio",nil,128,0)
 const video="youtube:https://www.youtube.com/watch?v=AMmBY2D9IJU"
 original:=broadcast.TrackInfo{Path:video,Title:"YouTube metadata pending"}
 frame:=bytes.Repeat([]byte{0x55},417)
 hub.Ring().WriteFrame(frame,&original)
 for i:=0;i<30;i++{hub.Ring().Write(frame)}
 hub.UpdateTrackMetadata(video,"THIRST","MAYHEM","")
 stream:=&Stream{Hub:hub,MetaInt:8192,Name:"Radio"}
 srv:=httptest.NewServer(stream)
 defer srv.Close()
 client:=&http.Client{Timeout:3*time.Second}
 request,err:=http.NewRequest("GET",srv.URL,nil);if err!=nil{t.Fatal(err)}
 request.Header.Set("Icy-MetaData","1")
 request.Header.Set("User-Agent","NekoSuneAI/1.0")
 response,err:=client.Do(request);if err!=nil{t.Fatal(err)}
 defer response.Body.Close()
 if response.StatusCode!=200{t.Fatalf("status %d",response.StatusCode)}
 interval,err:=strconv.Atoi(response.Header.Get("icy-metaint"))
 if err!=nil||interval!=8192{t.Fatalf("bad interval %v %v",interval,err)}
 if _,err:=io.CopyN(io.Discard,response.Body,int64(interval));err!=nil{t.Fatal(err)}
 var size [1]byte
 if _,err:=io.ReadFull(response.Body,size[:]);err!=nil{t.Fatal(err)}
 payload:=make([]byte,int(size[0])*16)
 if _,err:=io.ReadFull(response.Body,payload);err!=nil{t.Fatal(err)}
 metadata:=string(bytes.TrimRight(payload,"\x00"))
 if !strings.Contains(metadata,"StreamTitle='THIRST - MAYHEM';"){t.Fatalf("ICY title wrong: %q",metadata)}
 if strings.Contains(metadata,"youtube.com")||strings.Contains(metadata,"watch?v="){t.Fatalf("YouTube URL leaked in ICY: %q",metadata)}
}
