package broadcast

import (
 "bytes"
 "context"
 "testing"
)

func TestLiveMetadataUpdateDoesNotChangeMP3Bytes(t *testing.T){
 const url="youtube:https://www.youtube.com/watch?v=AMmBY2D9IJU"
 hub:=NewHub("radio",nil,64,0)
 old:=TrackInfo{Path:url,Title:"YouTube metadata pending"}
 hub.currentTrack.Store(old)
 audio:=bytes.Repeat([]byte{0xAB},417)
 hub.ring.WriteFrame(audio,&old)
 hub.UpdateTrackMetadata(url,"THIRST","MAYHEM","")
 current:=hub.CurrentTrack()
 if current.Artist!="THIRST"||current.Title!="MAYHEM"{t.Fatalf("current title stale: %+v",current)}
 read:=make([]byte,len(audio))
 n,_,track,err:=hub.ring.Read(context.Background(),0,read)
 if err!=nil{t.Fatal(err)}
 if n!=len(audio)||!bytes.Equal(read,audio){t.Fatal("metadata update changed audio bytes")}
 if track.Path!=url||track.Artist!="THIRST"||track.Title!="MAYHEM"{t.Fatalf("ring title stale: %+v",track)}
 // A different track must never inherit this metadata.
 hub.UpdateTrackMetadata("youtube:https://www.youtube.com/watch?v=dQw4w9WgXcQ","Other","Song","")
 if hub.CurrentTrack().Title!="MAYHEM"{t.Fatal("unrelated metadata overwrote current title")}
}
