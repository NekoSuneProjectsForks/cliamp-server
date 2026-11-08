package library

import (
 "database/sql"
 "errors"
 "os"
 "path/filepath"
 "strings"
 "sync"
 "time"
)

// ArtworkCacheEntry identifies one real source file, S3 object, remote file,
// or canonical YouTube video and the metadata currently associated with it.
// Cached misses are intentional: only an admin refresh retries them.
type ArtworkCacheEntry struct {
 Station string `json:"station"`
 SourceType string `json:"source_type"`
 SourceKey string `json:"source_key"`
 Artist string `json:"artist"`
 Title string `json:"title"`
 Album string `json:"album"`
 CoverURL string `json:"cover_url"`
 Status string `json:"status"`
 CheckedAt int64 `json:"checked_at"`
}

func ArtworkIdentity(station,path,artist,title,album string) ArtworkCacheEntry {
 kind,key:="local",path
 switch{
 case strings.HasPrefix(path,"youtube:"):
  kind="youtube"
  raw:=strings.TrimPrefix(path,"youtube:")
  if id:=YouTubeVideoID(raw);id!=""{key=id}else{key=raw}
 case strings.HasPrefix(path,"s3:"):kind="s3"
 case strings.HasPrefix(path,"https://")||strings.HasPrefix(path,"http://"):kind="remote"
 case path=="":kind="metadata";key=strings.ToLower(strings.TrimSpace(artist)+"\\x00"+strings.TrimSpace(title))
 }
 // Album is recorded but not used to discriminate: metadata sources can
 // populate it asynchronously without changing which audio file is playing.
 normalize:=func(s string)string{return strings.ToLower(strings.Join(strings.Fields(s)," "))}
 return ArtworkCacheEntry{Station:station,SourceType:kind,SourceKey:key,
  Artist:normalize(artist),Title:normalize(title),Album:strings.TrimSpace(album)}
}

func artworkCachePath()string {
 if s:=os.Getenv("CLIAMP_ARTWORK_CACHE_DB");s!=""{return s}
 return "/data/artwork-cache.sqlite"
}
var artworkDBMu sync.Mutex
var artworkConnection *sql.DB
var artworkConnectionPath string
func artworkDB()(*sql.DB,error){
 artworkDBMu.Lock();defer artworkDBMu.Unlock()
 path:=artworkCachePath()
 if artworkConnection!=nil&&artworkConnectionPath==path{return artworkConnection,nil}
 if artworkConnection!=nil{_ = artworkConnection.Close();artworkConnection=nil}
 if err:=os.MkdirAll(filepath.Dir(path),0700);err!=nil{return nil,err}
 f,err:=os.OpenFile(path,os.O_CREATE|os.O_RDWR,0600)
 if err!=nil{return nil,err}
 if err:=f.Close();err!=nil{return nil,err}
 db,err:=sql.Open("sqlite3",path)
 if err!=nil{return nil,err}
 db.SetMaxOpenConns(1)
 _,err=db.Exec(`CREATE TABLE IF NOT EXISTS artwork_cache(
 station TEXT NOT NULL,source_type TEXT NOT NULL,source_key TEXT NOT NULL,
 artist TEXT NOT NULL,title TEXT NOT NULL,album TEXT NOT NULL DEFAULT '',
 cover_url TEXT NOT NULL DEFAULT '',status TEXT NOT NULL,
 checked_at INTEGER NOT NULL,
 PRIMARY KEY(station,source_type,source_key,artist,title)
 )`)
 if err!=nil{db.Close();return nil,err}
 artworkConnection=db
 artworkConnectionPath=path
 return db,nil
}
func FindArtwork(entry ArtworkCacheEntry)(ArtworkCacheEntry,bool,error){
 db,err:=artworkDB();if err!=nil{return entry,false,err}
 err=db.QueryRow(`SELECT album,cover_url,status,checked_at FROM artwork_cache
 WHERE station=? AND source_type=? AND source_key=? AND artist=? AND title=?`,
 entry.Station,entry.SourceType,entry.SourceKey,entry.Artist,entry.Title).
 Scan(&entry.Album,&entry.CoverURL,&entry.Status,&entry.CheckedAt)
 if errors.Is(err,sql.ErrNoRows){return entry,false,nil}
 if err!=nil{return entry,false,err}
 return entry,true,nil
}
func SaveArtwork(entry ArtworkCacheEntry)error{
 if entry.Station==""||entry.SourceKey==""||entry.Status==""{return errors.New("invalid artwork cache identity")}
 db,err:=artworkDB();if err!=nil{return err}
 entry.CheckedAt=time.Now().Unix()
 _,err=db.Exec(`INSERT INTO artwork_cache
 (station,source_type,source_key,artist,title,album,cover_url,status,checked_at)
 VALUES(?,?,?,?,?,?,?,?,?)
 ON CONFLICT(station,source_type,source_key,artist,title)
 DO UPDATE SET album=excluded.album,cover_url=excluded.cover_url,
 status=excluded.status,checked_at=excluded.checked_at`,
 entry.Station,entry.SourceType,entry.SourceKey,entry.Artist,entry.Title,
 entry.Album,entry.CoverURL,entry.Status,entry.CheckedAt)
 return err
}
func ArtworkCacheStats(station string)(map[string]int,error){
 db,err:=artworkDB();if err!=nil{return nil,err}
 rows,err:=db.Query(`SELECT status,COUNT(*) FROM artwork_cache WHERE station=? GROUP BY status`,station)
 if err!=nil{return nil,err};defer rows.Close()
 result:=map[string]int{"found":0,"missing":0,"failed":0}
 for rows.Next(){var k string;var n int;if err:=rows.Scan(&k,&n);err!=nil{return nil,err};result[k]=n}
 return result,rows.Err()
}
