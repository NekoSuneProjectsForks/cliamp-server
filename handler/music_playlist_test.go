package handler

import (
 "encoding/json"
 "net/http"
 "net/http/httptest"
 "os"
 "path/filepath"
 "strings"
 "testing"

 "cliamp-server/library"
 "cliamp-server/playlist"
)

func TestOnDemandLibraryPlaylistIncludesYouTubeAndLocalWithoutOriginURLs(t *testing.T){
 dir:=t.TempDir()
 local:=filepath.Join(dir,"song.mp3")
 if err:=os.WriteFile(local,[]byte("local mp3 bytes"),0600);err!=nil{t.Fatal(err)}
 remote:="youtube:https://www.youtube.com/watch?v=abcDEF12345"
 tracks:=[]library.Track{
  {Path:local,Artist:"NEFFEX",Title:"Get Through"},
  {Path:remote,Artist:"NEFFEX",Title:"Built to Last"},
  {Path:"silence://standby",Title:"Awaiting music"},
 }
 live:=playlist.New(tracks,false)
 idx:=NewTrackIndex("neffex","Neffex Station",tracks)
 idx.Current=live.Snapshot
 request:=httptest.NewRequest(http.MethodGet,"https://radio.nekosunevr.co.uk/api/neffex/playlist",nil)
 response:=httptest.NewRecorder()
 (&TracksJSON{Index:idx}).ServeHTTP(response,request)
 if response.Code!=200{t.Fatal(response.Code)}
 var listing struct{
  Count int `json:"count"`
  Tracks []TrackEntry `json:"tracks"`
 }
 if err:=json.Unmarshal(response.Body.Bytes(),&listing);err!=nil{t.Fatal(err)}
 if listing.Count!=2{t.Fatalf("want 2 playable tracks, got %d",listing.Count)}
 if listing.Tracks[0].Source!="local"||listing.Tracks[1].Source!="youtube"{t.Fatal("wrong source classification")}
 for _,entry:=range listing.Tracks{
  if !strings.HasPrefix(entry.StreamURL,"https://radio.nekosunevr.co.uk/neffex/tracks/") || !strings.HasSuffix(entry.StreamURL,".mp3"){
   t.Fatalf("stream URL is not a stable server MP3 endpoint: %s",entry.StreamURL)
  }
  if strings.Contains(entry.StreamURL,"youtube.com")||strings.Contains(entry.StreamURL,"googlevideo") {t.Fatal("origin URL leaked")}
 }
 m3u:=httptest.NewRecorder()
 (&TracksM3U{Index:idx}).ServeHTTP(m3u,httptest.NewRequest(http.MethodGet,"https://radio.nekosunevr.co.uk/api/neffex/playlist.m3u",nil))
 if !strings.HasPrefix(m3u.Body.String(),"#EXTM3U\n")||strings.Count(m3u.Body.String(),"#EXTINF:")!=2{t.Fatalf("invalid M3U: %s",m3u.Body.String())}
 if strings.Contains(m3u.Body.String(),"youtube.com") || strings.Contains(m3u.Body.String(),"googlevideo.com"){t.Fatal("M3U exposed signed or direct YouTube URLs")}
 pls:=httptest.NewRecorder()
 (&TracksPLS{Index:idx}).ServeHTTP(pls,httptest.NewRequest(http.MethodGet,"https://radio.nekosunevr.co.uk/api/neffex/playlist.pls",nil))
 if !strings.HasPrefix(pls.Body.String(),"[playlist]\\n")||!strings.Contains(pls.Body.String(),"NumberOfEntries=2")||strings.Contains(pls.Body.String(),"youtube.com"){
  t.Fatalf("invalid or unsafe PLS: %s",pls.Body.String())
 }
 handler:=&TrackFile{Index:idx}
 localID:=listing.Tracks[0].ID
 served:=httptest.NewRecorder()
 r:=httptest.NewRequest(http.MethodGet,"https://radio.nekosunevr.co.uk/neffex/tracks/"+localID+".mp3",nil)
 r.SetPathValue("id",localID+".mp3")
 handler.ServeHTTP(served,r)
 if served.Code!=200||served.Header().Get("Accept-Ranges")!="bytes"{t.Fatalf("local MP3 lost range support: %d",served.Code)}
 // Playlist updates are reflected in API without server restart.
 if err:=live.Replace([]library.Track{{Path:local,Title:"Updated title"}});err!=nil{t.Fatal(err)}
 response=httptest.NewRecorder()
 (&TracksJSON{Index:idx}).ServeHTTP(response,request)
 if err:=json.Unmarshal(response.Body.Bytes(),&listing);err!=nil{t.Fatal(err)}
 if listing.Count!=1 || listing.Tracks[0].Title!="Updated title"{t.Fatalf("live library update not reflected: %+v",listing)}
}
func TestOnDemandLibraryUnknownTrackReturns404WithoutStartingProcess(t *testing.T){
 idx:=NewTrackIndex("neffex","Neffex",nil)
 rec:=httptest.NewRecorder()
 req:=httptest.NewRequest(http.MethodGet,"https://radio.nekosunevr.co.uk/neffex/tracks/invalid.mp3",nil)
 req.SetPathValue("id","invalid.mp3")
 (&TrackFile{Index:idx}).ServeHTTP(rec,req)
 if rec.Code!=http.StatusNotFound{t.Fatalf("expected 404, got %d",rec.Code)}
}
