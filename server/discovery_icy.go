package server

import (
 "context"
 "encoding/json"
 "errors"
 "fmt"
 "io"
 "net"
 "net/http"
 "net/url"
 "regexp"
 "strconv"
 "strings"
 "sync"
 "time"
)

var radioUUIDPattern=regexp.MustCompile(`^[a-fA-F0-9-]{20,50}$`)

type externalTrack struct {
 Title string `json:"title"`
 Artist string `json:"artist"`
 Raw string `json:"raw"`
 Available bool `json:"available"`
}
type cachedExternalTrack struct {value externalTrack;expires time.Time}
type radioNowPlaying struct {mu sync.Mutex;cache map[string]cachedExternalTrack;lookup *http.Client;stream *http.Client}
func newRadioNowPlaying()*radioNowPlaying{
 dialer:=&net.Dialer{Timeout:3*time.Second}
 safeTransport:=&http.Transport{
  Proxy:nil,
  DisableKeepAlives:true,
  ResponseHeaderTimeout:5*time.Second,
  DialContext:func(ctx context.Context,network,address string)(net.Conn,error){
   host,port,err:=net.SplitHostPort(address);if err!=nil{return nil,err}
   ips,err:=net.DefaultResolver.LookupIPAddr(ctx,host);if err!=nil{return nil,err}
   var allowed net.IP
   for _,ip:=range ips {if isPublicRadioIP(ip.IP){allowed=ip.IP;break}}
   if allowed==nil{return nil,errors.New("radio host resolves to a private or reserved address")}
   return dialer.DialContext(ctx,network,net.JoinHostPort(allowed.String(),port))
  },
 }
 return &radioNowPlaying{
  cache:map[string]cachedExternalTrack{},
  lookup:&http.Client{Timeout:5*time.Second},
  stream:&http.Client{Timeout:12*time.Second,Transport:safeTransport,CheckRedirect:func(req *http.Request,via []*http.Request)error{
   if len(via)>3{return errors.New("too many radio redirects")}
   if req.URL.Scheme!="https"&&req.URL.Scheme!="http"{return errors.New("invalid radio redirect")}
   return nil
  }},
 }
}
func isPublicRadioIP(ip net.IP)bool{
 if ip==nil||!ip.IsGlobalUnicast()||ip.IsPrivate()||ip.IsLoopback()||ip.IsLinkLocalUnicast()||ip.IsLinkLocalMulticast()||ip.IsMulticast()||ip.IsUnspecified(){return false}
 v4:=ip.To4()
 if v4!=nil{
  return v4[0]!=0&&v4[0]!=10&&v4[0]!=127&&v4[0]!=169&&v4[0]!=172&&v4[0]!=192&&v4[0]!=198&&v4[0]!=203&&v4[0]<224
 }
 // Exclude unique local, documentation and IPv4 mapped internal addresses.
 return !strings.HasPrefix(strings.ToLower(ip.String()),"2001:db8:")&&!strings.HasPrefix(strings.ToLower(ip.String()),"fc")&&!strings.HasPrefix(strings.ToLower(ip.String()),"fd")
}
func (n *radioNowPlaying) resolveStation(ctx context.Context,id string)(string,error){
 u:="https://de1.api.radio-browser.info/json/stations/byuuid/"+url.PathEscape(id)
 req,err:=http.NewRequestWithContext(ctx,"GET",u,nil);if err!=nil{return "",err}
 req.Header.Set("User-Agent","NekoSune-Radio/1.0")
 resp,err:=n.lookup.Do(req);if err!=nil{return "",err};defer resp.Body.Close()
 if resp.StatusCode!=200{return "",fmt.Errorf("directory status %d",resp.StatusCode)}
 var stations []struct {UUID string `json:"stationuuid"`;URL string `json:"url_resolved"`}
 if err:=json.NewDecoder(io.LimitReader(resp.Body,96<<10)).Decode(&stations);err!=nil{return "",err}
 for _,station:=range stations{
  if station.UUID!=id{continue}
  parsed,err:=url.Parse(station.URL)
  if err!=nil||parsed.Host==""||parsed.User!=nil||(parsed.Scheme!="https"&&parsed.Scheme!="http"){return "",errors.New("unusable radio URL")}
  return parsed.String(),nil
 }
 return "",errors.New("station not found")
}
func (n *radioNowPlaying) probe(ctx context.Context,id string)(externalTrack,error){
 raw,err:=n.resolveStation(ctx,id);if err!=nil{return externalTrack{},err}
 req,err:=http.NewRequestWithContext(ctx,"GET",raw,nil);if err!=nil{return externalTrack{},err}
 req.Header.Set("Icy-MetaData","1")
 req.Header.Set("User-Agent","NekoSuneRadio/1.0")
 resp,err:=n.stream.Do(req);if err!=nil{return externalTrack{},err}
 defer resp.Body.Close()
 if resp.StatusCode!=200{return externalTrack{},fmt.Errorf("stream status %d",resp.StatusCode)}
 interval,err:=strconv.Atoi(resp.Header.Get("icy-metaint"))
 if err!=nil||interval<1||interval>1024*1024{return externalTrack{},errors.New("station does not expose supported ICY metadata")}
 if _,err=io.CopyN(io.Discard,resp.Body,int64(interval));err!=nil{return externalTrack{},err}
 var size [1]byte
 if _,err=io.ReadFull(resp.Body,size[:]);err!=nil{return externalTrack{},err}
 if size[0]==0{return externalTrack{},errors.New("station supplied empty metadata block")}
 b:=make([]byte,int(size[0])*16)
 if _,err=io.ReadFull(resp.Body,b);err!=nil{return externalTrack{},err}
 line:=strings.TrimRight(string(b),"\x00")
 start:=strings.Index(line,"StreamTitle='")
 if start<0{return externalTrack{},errors.New("StreamTitle absent")}
 line=line[start+len("StreamTitle='"):]
 end:=strings.Index(line,"';")
 if end<0{return externalTrack{},errors.New("malformed StreamTitle")}
 title:=strings.TrimSpace(line[:end]);if title==""{return externalTrack{},errors.New("empty track title")}
 if len(title)>400{title=title[:400]}
 result:=externalTrack{Title:title,Raw:title,Available:true}
 if a,b,ok:=strings.Cut(title," - ");ok{result.Artist=strings.TrimSpace(a);result.Title=strings.TrimSpace(b)}
 return result,nil
}
func (n *radioNowPlaying) serve(w http.ResponseWriter,r *http.Request){
 id:=strings.TrimSpace(r.URL.Query().Get("id"))
 if !radioUUIDPattern.MatchString(id){http.Error(w,"Invalid station ID",400);return}
 n.mu.Lock();v,ok:=n.cache[id]
 if ok&&time.Now().Before(v.expires){n.mu.Unlock();writeExternalTrack(w,v.value);return}
 n.mu.Unlock()
 ctx,cancel:=context.WithTimeout(r.Context(),12*time.Second);defer cancel()
 track,err:=n.probe(ctx,id)
 if err!=nil{track=externalTrack{Title:"",Available:false}}
 n.mu.Lock();if len(n.cache)>1500{n.cache=make(map[string]cachedExternalTrack)}
 ttl:=12*time.Second;if err!=nil{ttl=30*time.Second}
 n.cache[id]=cachedExternalTrack{track,time.Now().Add(ttl)}
 n.mu.Unlock()
 writeExternalTrack(w,track)
}
func writeExternalTrack(w http.ResponseWriter,track externalTrack){
 w.Header().Set("Content-Type","application/json")
 w.Header().Set("Cache-Control","no-store")
 _=json.NewEncoder(w).Encode(track)
}
