package server

import (
 "encoding/json"
 "net/http"
 "io"
 "net/url"
 "strings"
 "regexp"
 "sync"
 "time"
)

type artworkEntry struct { URL string; Expiry time.Time }
type artworkLookup struct { mu sync.Mutex; entries map[string]artworkEntry; client *http.Client }

func newArtworkLookup()*artworkLookup{
 return &artworkLookup{entries:make(map[string]artworkEntry),client:&http.Client{Timeout:3*time.Second}}
}
var collaborationSplit=regexp.MustCompile(`(?i)\s+(?:feat\.?|ft\.?|featuring|with|x|&|and|,|;)\s+`)
var artworkPunctuation=regexp.MustCompile(`[^a-z0-9]+`)
func normalizeArtworkName(value string)string{
 return strings.TrimSpace(artworkPunctuation.ReplaceAllString(strings.ToLower(value)," "))
}
func primaryArtist(value string)string {
 parts:=collaborationSplit.Split(strings.TrimSpace(value),2)
 return strings.TrimSpace(parts[0])
}
func artworkMatch(candidateArtist,candidateTitle,requestedArtist,requestedTitle string) bool {
 if normalizeArtworkName(candidateTitle)!=normalizeArtworkName(requestedTitle){return false}
 primary:=normalizeArtworkName(primaryArtist(requestedArtist))
 actual:=normalizeArtworkName(candidateArtist)
 return primary!="" && (actual==primary||strings.HasPrefix(actual,primary+" "))
}
func(a *artworkLookup) cover(artist,title,album string)string{
 if strings.TrimSpace(artist)==""||strings.TrimSpace(title)==""{return ""}
 key:=strings.ToLower(strings.TrimSpace(artist)+"|"+strings.TrimSpace(title)+"|"+strings.TrimSpace(album))
 a.mu.Lock()
 cached,exists:=a.entries[key]
 if exists&&time.Now().Before(cached.Expiry){a.mu.Unlock();return cached.URL}
 a.mu.Unlock()
 term:=primaryArtist(artist)+" "+title
 endpoint:="https://itunes.apple.com/search?"+url.Values{"term":{term},"entity":{"song"},"limit":{"25"}}.Encode()
 response,err:=a.client.Get(endpoint)
 found:=""
 if err==nil{
  defer response.Body.Close()
  if response.StatusCode==http.StatusOK{
   var result struct{ Results []struct{ Artist string `json:"artistName"`; Track string `json:"trackName"`; Artwork string `json:"artworkUrl100"` } `json:"results"` }
   if json.NewDecoder(io.LimitReader(response.Body,128<<10)).Decode(&result)==nil{
    for _,candidate:=range result.Results{
     if artworkMatch(candidate.Artist,candidate.Track,artist,title){
      if parsed,e:=url.Parse(candidate.Artwork);e==nil&&parsed.Scheme=="https"&&strings.HasSuffix(strings.ToLower(parsed.Hostname()),"mzstatic.com"){
       found=strings.Replace(candidate.Artwork,"100x100bb","600x600bb",1)
      }
      break
     }
    }
   }
  }
 }
 a.mu.Lock();defer a.mu.Unlock()
 if len(a.entries)>3000 {a.entries=make(map[string]artworkEntry)}
 ttl:=6*time.Hour;if found!=""{ttl=24*time.Hour}
 a.entries[key]=artworkEntry{URL:found,Expiry:time.Now().Add(ttl)}
 return found
}
