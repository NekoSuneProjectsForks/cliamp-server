package server

import "testing"

func TestArtworkCollaborationCredits(t *testing.T) {
 cases:=[]struct{artist,title,requestedArtist,requestedTitle string; want bool}{
  {"Calvin Harris & Clementine Douglas","Blessings","Calvin Harris feat. Clementine Douglas","Blessings",true},
  {"Calvin Harris & Clementine Douglas","Blessings","Calvin Harris ft. Clementine Douglas","Blessings",true},
  {"Calvin Harris","Blessings","Calvin Harris","Blessings",true},
  {"Other Artist","Blessings","Calvin Harris feat. Clementine Douglas","Blessings",false},
  {"Calvin Harris & Clementine Douglas","Blessings (Extended Mix)","Calvin Harris feat. Clementine Douglas","Blessings",false},
 }
 for _,tt:=range cases {
  if got:=artworkMatch(tt.artist,tt.title,tt.requestedArtist,tt.requestedTitle);got!=tt.want {
   t.Errorf("artworkMatch(%q,%q,%q,%q)=%v want %v",tt.artist,tt.title,tt.requestedArtist,tt.requestedTitle,got,tt.want)
  }
 }
}
