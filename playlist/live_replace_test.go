package playlist

import (
 "testing"

 "cliamp-server/library"
)

func TestReplaceKeepsOnlyRemainingYouTubeTracks(t *testing.T) {
 tracks:=[]library.Track{
  {Path:"youtube:one",Title:"One"},
  {Path:"youtube:two",Title:"Two"},
  {Path:"youtube:three",Title:"Three"},
  {Path:"youtube:four",Title:"Four"},
  {Path:"youtube:five",Title:"Five"},
 }
 p:=New(tracks,false)
 oldRevision:=p.Revision()
 // Simulate the radio prefetching a track before a dashboard edit.
 prefetched:=p.Next()
 if prefetched.Path!="youtube:one"{t.Fatalf("unexpected prefetch %+v",prefetched)}
 kept:=[]library.Track{tracks[1],tracks[3],tracks[4]}
 if err:=p.Replace(kept);err!=nil{t.Fatal(err)}
 if p.Len()!=3{t.Fatalf("expected 3 tracks, got %d",p.Len())}
 if p.Revision()<=oldRevision{t.Fatal("replacement revision not advanced")}
 for _,path:=range []string{"youtube:one","youtube:three"}{
  if p.HasTrack(path){t.Fatalf("removed song still eligible: %s",path)}
 }
 for i:=0;i<15;i++{
  track:=p.Next()
  if !p.HasTrack(track.Path){t.Fatalf("removed track chosen: %q",track.Path)}
 }
 // Adding a song updates the same live playlist object.
 if err:=p.Replace(append(kept,tracks[2]));err!=nil{t.Fatal(err)}
 if !p.HasTrack("youtube:three")||p.Len()!=4{t.Fatal("added track did not join rotation")}
}
func TestReplacementDoesNotMutateCallerSlice(t *testing.T){
 tracks:=[]library.Track{{Path:"youtube:one"},{Path:"youtube:two"}}
 p:=New(tracks,false)
 if err:=p.Replace(tracks);err!=nil{t.Fatal(err)}
 tracks[0].Path="youtube:changed"
 if !p.HasTrack("youtube:one")||p.HasTrack("youtube:changed"){t.Fatal("playlist aliases mutable incoming slice")}
}
