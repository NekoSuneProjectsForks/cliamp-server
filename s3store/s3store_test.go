package s3store

import (
 "strings"
 "testing"
 "time"
)

func TestSignedURLAndTrackPath(t *testing.T) {
 c,err:=New(Config{Endpoint:"https://minio.example.com",Bucket:"songs",Region:"us-east-1",AccessKey:"access",SecretKey:"secret"})
 if err!=nil{t.Fatal(err)}
 target,err:=c.SignedURL("Music/Artist - Track.mp3",time.Hour)
 if err!=nil{t.Fatal(err)}
 for _,part:=range []string{"https://minio.example.com/songs/Music/","X-Amz-Signature=","X-Amz-Algorithm=AWS4-HMAC-SHA256"}{
 if !strings.Contains(target,part){t.Fatalf("signed URL does not contain %q: %s",part,target)}
 }
 p:=TrackPath("station","Music/Artist - Track.mp3")
 id,key,ok:=DecodePath(p)
 if !ok||id!="station"||key!="Music/Artist - Track.mp3"{t.Fatalf("roundtrip failed: %s %s %v",id,key,ok)}
}
func TestRejectInsecureEndpoint(t *testing.T) {
 if _,err:=New(Config{Endpoint:"http://minio.example.com",Bucket:"songs",AccessKey:"a",SecretKey:"b"});err==nil{t.Fatal("expected HTTPS validation")}
}
