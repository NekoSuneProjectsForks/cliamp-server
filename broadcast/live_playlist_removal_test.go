package broadcast

import (
 "testing"

 "cliamp-server/library"
 "cliamp-server/playlist"
)

func TestRemovedPrefetchedTrackRejectedWithoutChangingCurrentAudio(t *testing.T) {
 tracks:=[]library.Track{
  {Path:"youtube:keep-playing",Title:"Playing"},
  {Path:"youtube:remove-me",Title:"Removed"},
  {Path:"youtube:keep-next",Title:"Next"},
 }
 p:=playlist.New(tracks,false)
 hub:=NewHub("music",p,64,0)
 // The first two tracks were already selected before the admin update.
 current:=p.Next()
 prefetched:=p.Next()
 if current.Path!="youtube:keep-playing"||prefetched.Path!="youtube:remove-me"{t.Fatal("unexpected selection")}
 if err:=p.Replace([]library.Track{tracks[0],tracks[2]});err!=nil{t.Fatal(err)}
 if hub.trackStillQueued(prefetched){t.Fatal("removed prefetch was still accepted")}
 if !hub.trackStillQueued(current){t.Fatal("playing song should remain permitted")}
 next:=p.Next()
 if !hub.trackStillQueued(next)||next.Path=="youtube:remove-me"{t.Fatalf("old song selected: %+v",next)}
 // The removal must not modify the active hub's current track state.
 hub.currentTrack.Store(TrackInfo{Path:current.Path,Title:current.Title})
 if got:=hub.CurrentTrack().Path;got!=current.Path{t.Fatalf("playing track interrupted: %s",got)}
}
