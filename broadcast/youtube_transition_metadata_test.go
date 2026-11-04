package broadcast

import (
 "testing"

 "cliamp-server/library"
)

func TestLateMetadataResolverCorrectsPrefetchedYouTubeTrack(t *testing.T) {
 const path="youtube:https://www.youtube.com/watch?v=AMmBY2D9IJU"
 hub:=NewHub("monstercat-music",nil,64,0)
 prefetched:=library.Track{Path:path,Title:library.YouTubePendingTitle}
 hub.SetMetadataResolver(func(track library.Track)library.Track{
  track.Title="MAYHEM"
  track.Artist="THIRST"
  return track
 })
 resolved:=hub.resolveTransitionTrack(prefetched)
 if resolved.Title!="MAYHEM"||resolved.Artist!="THIRST"||resolved.Path!=path{
  t.Fatalf("publisher did not resolve metadata: %+v",resolved)
 }
 if prefetched.Title!=library.YouTubePendingTitle{t.Fatal("prefetched source was mutated")}
 // Metadata resolver cannot alter the playback source path.
 hub.SetMetadataResolver(func(track library.Track)library.Track{
  return library.Track{Path:"https://untrusted.example.invalid",Title:"ALREADY",Artist:"TEST"}
 })
 if got:=hub.resolveTransitionTrack(prefetched);got.Path!=path{t.Fatalf("resolver changed source path: %q",got.Path)}
}
