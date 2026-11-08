package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
 "context"
 "io"
 "cliamp-server/transcode"
	"fmt"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cliamp-server/geo"
	"cliamp-server/library"
	"cliamp-server/stats"
)

// audioMIME covers the extensions the library scanner accepts but that the
// system mime table often does not know about.
var audioMIME = map[string]string{
	".mp3":  "audio/mpeg",
	".wav":  "audio/wav",
	".flac": "audio/flac",
	".ogg":  "audio/ogg",
	".opus": "audio/ogg",
	".m4a":  "audio/mp4",
	".aac":  "audio/aac",
	".webm": "audio/webm",
	".wma":  "audio/x-ms-wma",
}

// TrackEntry is one track as exposed over HTTP. It deliberately omits the
// on-disk path so the filesystem layout is never published.
type TrackEntry struct {
 Source string `json:"source"`
 Seekable bool `json:"seekable"`
 StreamURL string `json:"stream_url"`
	ID       string  `json:"id"`
	Title    string  `json:"title"`
	Artist   string  `json:"artist,omitempty"`
	Album    string  `json:"album,omitempty"`
	Duration float64 `json:"duration,omitempty"`
	Filename string  `json:"filename"`
	URL      string  `json:"url"`
	Plays    int64   `json:"plays"`
}

// TrackIndex holds the per-station track listing and the id-to-path lookup
// used to serve individual files. The map is built once at startup, so a
// request can only ever name a track that was scanned into the library.
type TrackIndex struct {
	StationID   string
	StationName string

	entries []TrackEntry
	paths   map[string]string // track id -> absolute path
 Current func() []library.Track
}

// NewTrackIndex builds the exposed listing for one station.
func NewTrackIndex(stationID, stationName string, tracks []library.Track) *TrackIndex {
	idx := &TrackIndex{
		StationID:   stationID,
		StationName: stationName,
		entries:     make([]TrackEntry, 0, len(tracks)),
		paths:       make(map[string]string, len(tracks)),
	}

	for _, t := range tracks {
		if t.Path==""||t.Path=="silence://standby"{continue}
        id := trackID(t.Path)
		// A duplicate id means the same absolute path was scanned twice;
		// keep the first and skip the rest so ids stay unambiguous.
		if _, seen := idx.paths[id]; seen {
			continue
		}
		idx.paths[id] = t.Path
        kind:=trackSource(t.Path)
        filename:=filepath.Base(t.Path)
        switch kind {
        case "youtube":filename="YouTube video"
        case "remote":filename="Remote audio"
        case "s3":filename="S3 audio"
        }

		title := t.Title
		if title == "" {
			title = strings.TrimSuffix(filepath.Base(t.Path), filepath.Ext(t.Path))
		}

		idx.entries = append(idx.entries, TrackEntry{
			ID:       id,
			Title:    title,
			Artist:   t.Artist,
			Album:    t.Album,
			Duration: t.Duration,
			Filename: filename,
 Source: kind,
 Seekable: kind=="local",
		})
	}

	return idx
}

// Len reports how many tracks the index exposes.
func (idx *TrackIndex) Len() int { return len(idx.snapshot().entries) }
func (idx *TrackIndex) snapshot()*TrackIndex{
 if idx.Current!=nil{return NewTrackIndex(idx.StationID,idx.StationName,idx.Current())}
 return idx
}
func trackSource(path string)string{
 switch{
 case strings.HasPrefix(path,"youtube:"):return "youtube"
 case strings.HasPrefix(path,"s3://"):return "s3"
 case strings.HasPrefix(path,"https://")||strings.HasPrefix(path,"http://"):return "remote"
 default:return "local"
 }
}

// trackID derives a stable, opaque id from a track's absolute path. It is
// stable across restarts so links keep working, and it is not reversible into
// a path by a caller.
func trackID(path string) string {
	sum := sha256.Sum256([]byte(path))
	return hex.EncodeToString(sum[:8])
}

// baseURL returns the scheme and host the request arrived on, honouring a
// reverse proxy's X-Forwarded-Proto.
func baseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if fwd := r.Header.Get("X-Forwarded-Proto"); fwd != "" {
		scheme = fwd
	}
	return scheme + "://" + r.Host
}

// withURLs returns the entries with absolute URLs filled in for this request.
func (idx *TrackIndex) withURLs(r *http.Request, playCounts map[string]int64) []TrackEntry {
	base := baseURL(r) + "/" + idx.StationID + "/tracks/"
	out := make([]TrackEntry, len(idx.entries))
	for i, e := range idx.entries {
		e.URL = base + e.ID
        if e.Source!="local" {e.URL+=".mp3"}
        e.StreamURL=base+e.ID+".mp3"
		e.Plays = playCounts[e.ID]
		out[i] = e
	}
	return out
}

// TracksJSON handles GET /{station}/tracks - the track listing.
type TracksJSON struct {
	Index   *TrackIndex
	StatsDB *stats.DB
}

func (h *TracksJSON) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var playCounts map[string]int64
	if h.StatsDB != nil {
		var err error
		playCounts, err = h.StatsDB.TrackPlayCounts(h.Index.StationID)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	}
	entries := h.Index.snapshot().withURLs(r, playCounts)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(struct {
		Station string       `json:"station"`
		Name    string       `json:"name"`
		Count   int          `json:"count"`
		Tracks  []TrackEntry `json:"tracks"`
	}{
		Station: h.Index.StationID,
		Name:    h.Index.StationName,
		Count:   len(entries),
		Tracks:  entries,
	})
}

// TracksM3U handles GET /{station}/tracks.m3u - every song as a direct URL,
// so any player can open the station's library rather than the live stream.
type TracksM3U struct {
	Index *TrackIndex
}

func (h *TracksM3U) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	entries := h.Index.snapshot().withURLs(r, nil)

	w.Header().Set("Content-Type", "audio/x-mpegurl")
	w.Header().Set("Content-Disposition", "inline; filename=\"tracks.m3u\"")

	fmt.Fprint(w, "#EXTM3U\n")
	for _, e := range entries {
		label := e.Title
		if e.Artist != "" {
			label = e.Artist + " - " + e.Title
		}
		// -1 is the M3U convention for an unknown duration; the library
		// scanner does not decode files, so Duration is often 0.
		secs := -1
		if e.Duration > 0 {
			secs = int(e.Duration)
		}
		label=strings.ReplaceAll(strings.ReplaceAll(label,"\r"," "), "\n"," ")
        fmt.Fprintf(w, "#EXTINF:%d,%s\n%s\n", secs, label, e.StreamURL)
	}
}

// TrackFile handles GET /{station}/tracks/{id} - one audio file.
type TrackFile struct {
	Index     *TrackIndex
	StatsDB   *stats.DB
	GeoDB     *geo.DB         // Optional geo database (nil = no geo lookup)
	Listeners *TrackListeners // Optional active listener tracker
}

func (h *TrackFile) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSuffix(r.PathValue("id"),".mp3")

	path, ok := h.Index.snapshot().paths[id]
	if !ok {
		http.NotFound(w, r)
		return
	}

    transcoded:=strings.HasSuffix(r.PathValue("id"),".mp3") || trackSource(path)!="local"
    if transcoded {
        // Never reveal temporary yt-dlp/googlevideo URLs or S3 signatures to
        // users. Each request streams a stable server-hosted MP3 endpoint.
        serveOnDemandMP3(w,r,path)
        return
    }
	f, err := os.Open(path)
	if err != nil {
		// The library was scanned at startup; the file may have been moved
		// since. Report it as missing rather than leaking the reason.
		http.NotFound(w, r)
		return
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil || fi.IsDir() {
		http.NotFound(w, r)
		return
	}

	ext := strings.ToLower(filepath.Ext(path))
	ctype := audioMIME[ext]
	if ctype == "" {
		ctype = mime.TypeByExtension(ext)
	}
	if ctype == "" {
		ctype = "application/octet-stream"
	}

	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Accept-Ranges", "bytes")
	// Inline so browsers play it in place; the filename is still offered for
	// anyone who chooses to save it.
	w.Header().Set("Content-Disposition",
		fmt.Sprintf("inline; filename*=UTF-8''%s", urlEncode(filepath.Base(path))))

	if h.StatsDB != nil && isTrackPlayRequest(r) {
		h.recordPlay(r, id)
	}

	// ServeContent handles Range requests, If-Modified-Since and Content-Length.
	http.ServeContent(w, r, filepath.Base(path), fi.ModTime(), f)
}

// recordPlay stores one play of track id with the client's location.
func (h *TrackFile) recordPlay(r *http.Request, id string) {
	now := time.Now()
	ip := clientIP(r)

	var loc geo.Location
	if h.GeoDB != nil {
		loc = h.GeoDB.Lookup(ip)
	}

	play := stats.TrackPlay{
		Station:     h.Index.StationID,
		TrackID:     id,
		Country:     loc.Country,
		CountryCode: loc.CountryCode,
		City:        loc.City,
		Latitude:    loc.Latitude,
		Longitude:   loc.Longitude,
		PlayedAt:    now,
	}
	if h.Listeners != nil {
		var repeat bool
		play.Listeners, play.TotalListeners, repeat = h.Listeners.Touch(h.Index.StationID, ip, id, loc, now)
		if repeat {
			return
		}
	}

	if err := h.StatsDB.RecordTrackPlay(play); err != nil {
		slog.Error("failed to record track play", "station", h.Index.StationID, "track_id", id, "error", err)
	}
}

// isTrackPlayRequest excludes HEAD requests and seeks into the middle of a
// track from the play count.
func isTrackPlayRequest(r *http.Request) bool {
	if r.Method != http.MethodGet {
		return false
	}
	rangeHeader := strings.ToLower(strings.TrimSpace(r.Header.Get("Range")))
	return rangeHeader == "" || strings.HasPrefix(rangeHeader, "bytes=0-")
}

// urlEncode percent-encodes a filename for a Content-Disposition header.
func urlEncode(s string) string {
	var b strings.Builder
	for _, c := range []byte(s) {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') ||
			c == '-' || c == '.' || c == '_' || c == '~' {
			b.WriteByte(c)
			continue
		}
		fmt.Fprintf(&b, "%%%02X", c)
	}
	return b.String()
}

// Limit expensive per-listener encoders on Raspberry Pi hardware. This does
// not affect the continuously running live station encoder.
var onDemandSlots=make(chan struct{},4)
var youtubeSlots=make(chan struct{},2)
func serveOnDemandMP3(w http.ResponseWriter,r *http.Request,path string){
 w.Header().Set("Content-Type","audio/mpeg")
 w.Header().Set("Cache-Control","no-store")
 w.Header().Set("Accept-Ranges","none")
 w.Header().Set("X-Content-Type-Options","nosniff")
 if r.Method==http.MethodHead{return}
 // Signed and expiring upstream links are resolved inside ffmpeg/yt-dlp;
 // the public URL is stable, but HTTP byte-range seeking is not available.
 select{case onDemandSlots<-struct{}{}:defer func(){<-onDemandSlots}()
 default:http.Error(w,"On-demand encoders busy",http.StatusServiceUnavailable);return}
 isYouTube:=strings.HasPrefix(path,"youtube:")
 if isYouTube{
  select{case youtubeSlots<-struct{}{}:defer func(){<-youtubeSlots}()
  default:http.Error(w,"YouTube on-demand limit reached",http.StatusServiceUnavailable);return}
 }
 ctx,cancel:=context.WithCancel(r.Context())
 defer cancel()
 reader,err:=transcode.NewReader(ctx,path)
 if err!=nil{http.Error(w,"Audio source currently unavailable",http.StatusBadGateway);return}
 defer reader.Close()
 first:=make([]byte,32*1024)
 n,e:=reader.Read(first)
 if n==0{
  if e!=nil{http.Error(w,"Audio source could not start",http.StatusBadGateway)}
  return
 }
 w.WriteHeader(http.StatusOK)
 if _,err:=w.Write(first[:n]);err!=nil{return}
 _,_=io.Copy(w,reader)
}
