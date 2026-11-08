package library

import (
 "os"
 "path/filepath"
 "testing"
)

func TestArtworkDatabasePersistsSeparateSourcesAndMisses(t *testing.T){
 path:=filepath.Join(t.TempDir(),"artwork-cache.sqlite")
 t.Setenv("CLIAMP_ARTWORK_CACHE_DB",path)
 first:=ArtworkIdentity("neffex","youtube:https://www.youtube.com/watch?v=abcDEF12345","NEFFEX","Get Through","Album")
 second:=ArtworkIdentity("neffex","/music/other.mp3","NEFFEX","Get Through","Album")
 third:=ArtworkIdentity("chill","youtube:https://www.youtube.com/watch?v=abcDEF12345","NEFFEX","Get Through","Album")
 if first.SourceType!="youtube"||first.SourceKey!="abcDEF12345"{t.Fatalf("incorrect YouTube identity %+v",first)}
 if second.SourceType!="local"||second.SourceKey==first.SourceKey{t.Fatal("local file reused YouTube identity")}
 if err:=SaveArtwork(ArtworkCacheEntry{Station:first.Station,SourceType:first.SourceType,SourceKey:first.SourceKey,Artist:first.Artist,Title:first.Title,Album:first.Album,CoverURL:"https://is1-ssl.mzstatic.com/cover.jpg",Status:"found"});err!=nil{t.Fatal(err)}
 if err:=SaveArtwork(ArtworkCacheEntry{Station:second.Station,SourceType:second.SourceType,SourceKey:second.SourceKey,Artist:second.Artist,Title:second.Title,Status:"missing"});err!=nil{t.Fatal(err)}
 cached,ok,err:=FindArtwork(first)
 if err!=nil||!ok||cached.Status!="found"{t.Fatalf("missing persisted YouTube hit: %+v %v %v",cached,ok,err)}
 local,ok,err:=FindArtwork(second)
 if err!=nil||!ok||local.Status!="missing"{t.Fatalf("missing persisted local negative result: %+v %v %v",local,ok,err)}
 _,found,err:=FindArtwork(third)
 if err!=nil||found{t.Fatalf("different station must not cross-match another station entry: %v %v",found,err)}
 counts,err:=ArtworkCacheStats("neffex")
 if err!=nil||counts["found"]!=1||counts["missing"]!=1{t.Fatalf("incorrect cache counts: %v %v",counts,err)}
 if _,err:=os.Stat(path);err!=nil{t.Fatalf("SQLite database was not created: %v",err)}
 update:=first
 update.Album="Album updated after playback"
 cached,found,err=FindArtwork(update)
 if err!=nil||!found{t.Fatalf("album metadata update must preserve source identity: %v",err)}
 changed:=ArtworkIdentity("neffex","/music/other.mp3","Different Artist","Get Through","Album")
 _,found,err=FindArtwork(changed)
 if err!=nil||found{t.Fatal("mismatched artist should not reuse another song's cover")}
}

func TestArtworkCacheCanonicalYouTubeIdentity(t *testing.T){
 a:=ArtworkIdentity("neffex","youtube:https://youtu.be/abcDEF12345","Artist","Song","")
 b:=ArtworkIdentity("neffex","youtube:https://www.youtube.com/watch?v=abcDEF12345","Artist","Song","")
 if a.SourceKey!=b.SourceKey || a.SourceType!="youtube"{t.Fatal("same YouTube video must share source identity")}
}
