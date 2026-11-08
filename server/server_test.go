package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"cliamp-server/broadcast"
	"cliamp-server/config"
	"cliamp-server/library"
	"cliamp-server/stats"
)

func TestLogoEndpoint(t *testing.T) {
	srv := newTestServer()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "https://radio.example/logo.svg", nil)
	srv.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); got != "image/svg+xml" {
		t.Errorf("Content-Type = %q, want %q", got, "image/svg+xml")
	}
	if got := rec.Header().Get("Cache-Control"); got != "public, max-age=86400" {
		t.Errorf("Cache-Control = %q, want %q", got, "public, max-age=86400")
	}
	if !strings.Contains(rec.Header().Get("Access-Control-Expose-Headers"), "Icy-Logo") {
		t.Error("Access-Control-Expose-Headers does not include Icy-Logo")
	}
	if !strings.Contains(rec.Body.String(), "<svg") {
		t.Error("response does not contain an SVG")
	}
}

func TestStatusIncludesLogoURL(t *testing.T) {
	cfg := config.Defaults()
 cfg.Admin.Password = "test-secret"
 cfg.Stations = map[string]config.StationConfig{"omarchy": {Name:"Omarchy"}}
 cfg.StationOrder = []string{"omarchy"}
 stations := map[string]*Station{"omarchy": {Hub: broadcast.NewHub("omarchy", nil, 64, 0), Config: cfg.Stations["omarchy"]}}
 srv := New(cfg,stations,nil,nil)

	t.Run("station", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "https://radio.example/omarchy/status", nil)
		req.Header.Set("Authorization","Bearer test-secret")
  srv.httpServer.Handler.ServeHTTP(rec, req)

		var got struct {
			Favicon string `json:"favicon"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		if want := "https://radio.example/logo.svg"; got.Favicon != want {
			t.Errorf("favicon = %q, want %q", got.Favicon, want)
		}
	})

	t.Run("global", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "https://radio.example/status", nil)
		req.Header.Set("Authorization","Bearer test-secret")
  srv.httpServer.Handler.ServeHTTP(rec, req)

		var got struct {
			Stations map[string]struct {
				Favicon string `json:"favicon"`
			} `json:"stations"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		if want := "https://radio.example/logo.svg"; got.Stations["omarchy"].Favicon != want {
			t.Errorf("favicon = %q, want %q", got.Stations["omarchy"].Favicon, want)
		}
	})
}

func TestTrackStatisticsRoutes(t *testing.T) {
	db, err := stats.Open(filepath.Join(t.TempDir(), "stats.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	tests := []struct {
		name    string
		statsDB *stats.DB
		want    int
	}{
		{name: "with statistics", statsDB: db, want: http.StatusOK},
		{name: "without statistics", statsDB: nil, want: http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.Defaults()
			cfg.Stations = map[string]config.StationConfig{
				"omarchy": {Name: "Omarchy", ExposeTracks: true},
			}
			cfg.StationOrder = []string{"omarchy"}
			stations := map[string]*Station{
				"omarchy": {
					Hub:    broadcast.NewHub("omarchy", nil, 64, 0),
					Config: cfg.Stations["omarchy"],
					Tracks: []library.Track{{Path: "/music/song.mp3", Title: "Song"}},
				},
			}
			srv := New(cfg, stations, nil, tt.statsDB)

			for _, path := range []string{"/omarchy/tracks/statistics", "/tracks/statistics"} {
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, path, nil)
				srv.httpServer.Handler.ServeHTTP(rec, req)
				if rec.Code != tt.want {
					t.Errorf("GET %s status = %d, want %d", path, rec.Code, tt.want)
				}
			}
		})
	}
}

func TestStationsRouteUsesConfigOrder(t *testing.T) {
	cfg := config.Defaults()
	cfg.Stations = map[string]config.StationConfig{
		"omarchy": {Name: "Omarchy", ExposeTracks: true},
		"lofi":    {Name: "Lofi"},
	}
	cfg.StationOrder = []string{"lofi", "omarchy"}
	stations := map[string]*Station{
		"omarchy": {
			Hub:    broadcast.NewHub("omarchy", nil, 64, 0),
			Config: cfg.Stations["omarchy"],
			Tracks: []library.Track{{Path: "/music/a.mp3"}, {Path: "/music/b.mp3"}},
		},
		"lofi": {
			Hub:    broadcast.NewHub("lofi", nil, 64, 0),
			Config: cfg.Stations["lofi"],
		},
	}
	srv := New(cfg, stations, nil, nil)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "https://radio.example/stations", nil)
	srv.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var got struct {
		Stations []struct {
			ID        string `json:"id"`
			Tracks    int    `json:"tracks"`
			TracksURL string `json:"tracks_url"`
            LibraryEnabled bool `json:"library_enabled"`
            TrackStatisticsEnabled bool `json:"track_statistics_enabled"`
		} `json:"stations"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if len(got.Stations) != 2 || got.Stations[0].ID != "lofi" || got.Stations[1].ID != "omarchy" {
		t.Fatalf("stations = %#v, want lofi then omarchy", got.Stations)
	}
	if got.Stations[0].Tracks != 0 || got.Stations[0].TracksURL != "" {
		t.Errorf("lofi = %#v, want no tracks", got.Stations[0])
	}
	if got.Stations[1].Tracks != 2 || got.Stations[1].TracksURL != "https://radio.example/omarchy/tracks" || !got.Stations[1].LibraryEnabled || got.Stations[1].TrackStatisticsEnabled {
		t.Errorf("omarchy = %#v, want 2 tracks with a tracks URL", got.Stations[1])
	}
}

func TestStationsTrackCountNotHiddenWhenLibrarySharingDisabled(t *testing.T) {
 cfg:=config.Defaults()
 cfg.Stations=map[string]config.StationConfig{
   "neffex":{Name:"Neffex Station",Source:"youtube",YouTubeURLs:[]string{"https://www.youtube.com/watch?v=abcDEF12345","https://www.youtube.com/watch?v=defABC12345"}},
   "local":{Name:"Local Station"},
 }
 cfg.StationOrder=[]string{"neffex","local"}
 stations:=map[string]*Station{
  "neffex":{ID:"neffex",Config:cfg.Stations["neffex"],Hub:broadcast.NewHub("neffex",nil,64,0),Tracks:[]library.Track{{Path:"silence://standby",Title:"Awaiting music"}}},
  "local":{ID:"local",Config:cfg.Stations["local"],Hub:broadcast.NewHub("local",nil,64,0),Tracks:[]library.Track{{Path:"/music/first.mp3"},{Path:"/music/second.mp3"}}},
 }
 srv:=New(cfg,stations,nil,nil)
 rec:=httptest.NewRecorder()
 srv.httpServer.Handler.ServeHTTP(rec,httptest.NewRequest(http.MethodGet,"https://radio.example/stations",nil))
 var got struct{Stations []struct {
  ID string `json:"id"`
  Tracks int `json:"tracks"`
  TracksURL string `json:"tracks_url"`
  LibraryEnabled bool `json:"library_enabled"`
  TrackStatisticsEnabled bool `json:"track_statistics_enabled"`
 } `json:"stations"`}
 if err:=json.NewDecoder(rec.Body).Decode(&got);err!=nil{t.Fatal(err)}
 if len(got.Stations)!=2{t.Fatalf("stations count %d",len(got.Stations))}
 for _,station:=range got.Stations{
  if station.Tracks!=2{t.Errorf("%s count=%d expected 2 even with sharing disabled",station.ID,station.Tracks)}
  if station.LibraryEnabled||station.TrackStatisticsEnabled||station.TracksURL!=""{t.Errorf("%s leaked disabled library route: %+v",station.ID,station)}
 }
 noTracks:=httptest.NewRecorder()
 srv.httpServer.Handler.ServeHTTP(noTracks,httptest.NewRequest(http.MethodGet,"/neffex/tracks",nil))
 if noTracks.Code!=http.StatusNotFound{t.Fatalf("expected disabled library 404, got %d",noTracks.Code)}
}

func newTestServer() *Server {
	cfg := config.Defaults()
	cfg.Stations = map[string]config.StationConfig{
		"omarchy": {Name: "Omarchy"},
	}
	cfg.StationOrder = []string{"omarchy"}
	stations := map[string]*Station{
		"omarchy": {
			Hub:    broadcast.NewHub("omarchy", nil, 64, 0),
			Config: cfg.Stations["omarchy"],
		},
	}
	return New(cfg, stations, nil, nil)
}
