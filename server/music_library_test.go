package server

import (
 "bytes"
 "encoding/json"
 "net/http"
 "net/http/httptest"
 "os"
 "path/filepath"
 "strings"
 "testing"

 "cliamp-server/broadcast"
 "cliamp-server/config"
 "cliamp-server/library"
 "cliamp-server/playlist"
 "cliamp-server/vrchathls"
)

func TestMusicLibraryOptInRoutesAndSeparateSongArtwork(t *testing.T){
 old:=vrchathls.AssetsDirectory
 vrchathls.AssetsDirectory=t.TempDir()
 t.Cleanup(func(){vrchathls.AssetsDirectory=old})
 t.Setenv("CLIAMP_ARTWORK_CACHE_DB",filepath.Join(t.TempDir(),"art.sqlite"))
 dir:=t.TempDir()
 song:=filepath.Join(dir,"track.mp3")
 if err:=os.WriteFile(song,[]byte("fake mp3"),0600);err!=nil{t.Fatal(err)}
 youtube:="youtube:https://www.youtube.com/watch?v=abcDEF12345"
 tracks:=[]library.Track{{Path:song,Artist:"Example",Title:"Local"},{Path:youtube,Artist:"NEFFEX",Title:"Get Through"}}
 cfg:=config.Defaults()
 cfg.Stations=map[string]config.StationConfig{
  "neffex":{Name:"Neffex Station",Source:"youtube",ExposeTracks:true},
 }
 cfg.StationOrder=[]string{"neffex"}
 st:=&Station{ID:"neffex",Config:cfg.Stations["neffex"],Hub:broadcast.NewHub("neffex",nil,64,0),Tracks:tracks,Playlist:playlist.New(tracks,false)}
 srv:=New(cfg,map[string]*Station{"neffex":st},nil,nil)
 call:=func(path string)*httptest.ResponseRecorder{
  w:=httptest.NewRecorder();r:=httptest.NewRequest(http.MethodGet,"https://radio.nekosunevr.co.uk"+path,nil)
  srv.httpServer.Handler.ServeHTTP(w,r)
  return w
 }
 list:=call("/api/neffex/playlist")
 if list.Code!=200{t.Fatalf("playlist status %d: %s",list.Code,list.Body.String())}
 var data struct{Count int `json:"count"`;Tracks []struct{ID string `json:"id"`;Source string `json:"source"`;CoverURL string `json:"cover_url"`;StreamURL string `json:"stream_url"` } `json:"tracks"`}
 if err:=json.Unmarshal(list.Body.Bytes(),&data);err!=nil{t.Fatal(err)}
 if data.Count!=2{t.Fatalf("playlist expected 2 tracks: %d",data.Count)}
 if data.Tracks[1].Source!="youtube"{t.Fatalf("missing source type %+v",data.Tracks)}
 if !strings.HasPrefix(data.Tracks[1].CoverURL,"https://radio.nekosunevr.co.uk/api/neffex/playlist/"){t.Fatalf("unexpected song art URL: %s",data.Tracks[1].CoverURL)}
 art:=call("/api/neffex/playlist/"+data.Tracks[1].ID+"/art.png")
 if art.Code!=200||art.Header().Get("Content-Type")!="image/png"||!bytes.HasPrefix(art.Body.Bytes(),[]byte{137,80,78,71}){t.Fatalf("no CD art for uncached song (%d)",art.Code)}
 if call("/api/neffex/playlist/nonexistent/art.png").Code!=404{t.Fatal("unknown track artwork must be 404")}
 if call("/library/neffex").Code!=200{t.Fatal("browser library page unavailable")}
 m3u:=call("/api/neffex/playlist.m3u")
 if m3u.Code!=200||strings.Count(m3u.Body.String(),"#EXTINF:")!=2{t.Fatalf("invalid on-demand playlist: %s",m3u.Body.String())}
 if strings.Contains(m3u.Body.String(),"youtube.com")||strings.Contains(m3u.Body.String(),"googlevideo"){t.Fatal("upstream URL leaked in playlist")}
 public:=call("/api/public/radio")
 if !strings.Contains(public.Body.String(),"\"playlist_url\":\"/library/neffex\""){t.Fatal("playlist link missing on station directory")}
 disabled:=config.Defaults()
 disabled.Stations=map[string]config.StationConfig{"neffex":{Name:"Neffex",ExposeTracks:false}}
 disabled.StationOrder=[]string{"neffex"}
 disabledStation:=&Station{ID:"neffex",Config:disabled.Stations["neffex"],Hub:broadcast.NewHub("neffex",nil,64,0),Tracks:tracks,Playlist:playlist.New(tracks,false)}
 other:=New(disabled,map[string]*Station{"neffex":disabledStation},nil,nil)
 blocked:=httptest.NewRecorder()
 other.httpServer.Handler.ServeHTTP(blocked,httptest.NewRequest(http.MethodGet,"/api/neffex/playlist",nil))
 if blocked.Code!=404{t.Fatalf("unshared library exposed (%d)",blocked.Code)}
}
