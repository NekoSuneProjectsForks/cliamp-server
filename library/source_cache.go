package library

import (
 "database/sql"
 "errors"
 "net/url"
 "os"
 "path/filepath"
 "strings"
 "sync"

 _ "github.com/ncruces/go-sqlite3/driver"
 _ "github.com/ncruces/go-sqlite3/embed"
)

// SourceCacheEntry is persisted once for each canonical YouTube video ID.
// Station-specific metadata overrides remain in the existing track library.
type SourceCacheEntry struct {
 Title string `json:"title"`
 Artist string `json:"artist"`
 Album string `json:"album,omitempty"`
 OriginalTitle string `json:"original_title"`
 ArtworkChecked bool `json:"artwork_checked"`
 ArtworkURL string `json:"artwork_url,omitempty"`
}
var sourceCacheMu sync.Mutex
var cacheDB *sql.DB
var cacheDBPath string
func sourceCacheFile()string{
 if p:=os.Getenv("CLIAMP_SOURCE_CACHE_FILE");p!=""{return p}
 return "/data/youtube-source-cache.sqlite"
}
// YouTubeVideoID accepts watch URLs from the supported hosts and short links.
func YouTubeVideoID(raw string)string{
 u,err:=url.Parse(raw);if err!=nil||u.Scheme!="https"||u.User!=nil||u.Port()!=""{return ""}
 var id string
 switch strings.ToLower(u.Hostname()){
 case "youtube.com","www.youtube.com","m.youtube.com","music.youtube.com":
  if u.Path!="/watch"{return ""};id=u.Query().Get("v")
 case "youtu.be","www.youtu.be":
  if u.Path==""||strings.Count(u.Path,"/")!=1{return ""}
  id=strings.TrimPrefix(u.Path,"/")
 default:return ""
 }
 if len(id)!=11{return ""}
 for _,r:=range id {if !(r>='a'&&r<='z'||r>='A'&&r<='Z'||r>='0'&&r<='9'||r=='-'||r=='_'){return ""}}
 return id
}
func sourceDB()(*sql.DB,error){
 path:=sourceCacheFile()
 if cacheDB!=nil&&cacheDBPath==path{return cacheDB,nil}
 if cacheDB!=nil{_ = cacheDB.Close();cacheDB=nil}
 if err:=os.MkdirAll(filepath.Dir(path),0700);err!=nil{return nil,err}
 // Restrict SQLite cache permissions to the running radio user.
 file,err:=os.OpenFile(path,os.O_CREATE|os.O_RDWR,0600)
 if err!=nil{return nil,err}
 _=file.Close()
 db,err:=sql.Open("sqlite3",path)
 if err!=nil{return nil,err}
 db.SetMaxOpenConns(1)
 _,err=db.Exec(`CREATE TABLE IF NOT EXISTS youtube_source_cache (
  video_id TEXT PRIMARY KEY,
  title TEXT NOT NULL,
  artist TEXT NOT NULL,
  album TEXT NOT NULL DEFAULT '',
  original_title TEXT NOT NULL,
  artwork_checked INTEGER NOT NULL DEFAULT 0,
  artwork_url TEXT NOT NULL DEFAULT ''
 )`)
 if err!=nil{db.Close();return nil,err}
 cacheDB=db;cacheDBPath=path
 return db,nil
}
func CachedYouTube(raw string)(SourceCacheEntry,bool,error){
 id:=YouTubeVideoID(raw);if id==""{return SourceCacheEntry{},false,errors.New("invalid YouTube URL")}
 sourceCacheMu.Lock();defer sourceCacheMu.Unlock()
 db,err:=sourceDB();if err!=nil{return SourceCacheEntry{},false,err}
 var entry SourceCacheEntry
 var checked int
 err=db.QueryRow(`SELECT title,artist,album,original_title,artwork_checked,artwork_url FROM youtube_source_cache WHERE video_id=?`,id).
  Scan(&entry.Title,&entry.Artist,&entry.Album,&entry.OriginalTitle,&checked,&entry.ArtworkURL)
 if errors.Is(err,sql.ErrNoRows){return SourceCacheEntry{},false,nil}
 if err!=nil{return SourceCacheEntry{},false,err}
 entry.ArtworkChecked=checked!=0
 return entry,true,nil
}
func SaveCachedYouTube(raw string,entry SourceCacheEntry)error{
 id:=YouTubeVideoID(raw);if id==""||strings.TrimSpace(entry.Title)==""{return errors.New("invalid cached YouTube metadata")}
 sourceCacheMu.Lock();defer sourceCacheMu.Unlock()
 db,err:=sourceDB();if err!=nil{return err}
 checked:=0;if entry.ArtworkChecked{checked=1}
 _,err=db.Exec(`INSERT INTO youtube_source_cache(video_id,title,artist,album,original_title,artwork_checked,artwork_url)
 VALUES(?,?,?,?,?,?,?)
 ON CONFLICT(video_id) DO UPDATE SET title=excluded.title,artist=excluded.artist,
 album=excluded.album,original_title=excluded.original_title,
 artwork_checked=excluded.artwork_checked,artwork_url=excluded.artwork_url`,
 id,entry.Title,entry.Artist,entry.Album,entry.OriginalTitle,checked,entry.ArtworkURL)
 return err
}
// ApplySourceCache fills gaps only; manual per-station edits always win.
func ApplySourceCache(station string,tracks []Track)error{
 for i:=range tracks{
  if !strings.HasPrefix(tracks[i].Path,"youtube:"){continue}
  if _,saved,err:=MetadataFor(station,tracks[i].Path);err!=nil{return err}else if saved{continue}
  entry,ok,err:=CachedYouTube(strings.TrimPrefix(tracks[i].Path,"youtube:"))
  if err!=nil{return err}
  if ok&&entry.Title!=""{
   tracks[i].Title=entry.Title;tracks[i].Artist=entry.Artist;tracks[i].Album=entry.Album
  }
 }
 return nil
}

// ResolveLatestYouTubeTrack applies the newest persisted metadata at the
// moment the track is published. A prefetched track can still carry the
// pending placeholder while background yt-dlp writes to the cache.
// Manual per-station edits take precedence over shared video-ID records.
func ResolveLatestYouTubeTrack(station string,track Track) Track {
 if !strings.HasPrefix(track.Path,"youtube:"){return track}
 if manual,found,err:=MetadataFor(station,track.Path);err==nil&&found&&manual.Title!=""{
  track.Title=manual.Title
  track.Artist=manual.Artist
  track.Album=manual.Album
  return track
 }
 entry,found,err:=CachedYouTube(strings.TrimPrefix(track.Path,"youtube:"))
 if err==nil&&found&&entry.Title!=""{
  track.Title=entry.Title
  track.Artist=entry.Artist
  track.Album=entry.Album
 }
 return track
}
