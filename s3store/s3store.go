package s3store

import (
 "context"
 "crypto/hmac"
 "crypto/sha256"
 "encoding/hex"
 "encoding/xml"
 "errors"
 "fmt"
 "io"
 "net/http"
 "net/url"
 "path"
 "sort"
 "strings"
 "sync"
 "time"
)

type Config struct {
 Endpoint string
 Region string
 Bucket string
 Prefix string
 AccessKey string
 SecretKey string
}
type Object struct{ Key string; Size int64 }
type Client struct { cfg Config; base *url.URL; http *http.Client }
var registry sync.Map
func Register(station string,cfg Config)error {c,err:=New(cfg);if err!=nil{return err};registry.Store(station,c);return nil}
func ClientFor(station string)(*Client,bool){v,ok:=registry.Load(station);if !ok{return nil,false};return v.(*Client),true}
func TrackPath(station,key string)string{return "s3://"+station+"/"+url.PathEscape(key)}
func DecodePath(p string)(station,key string,ok bool){u,err:=url.Parse(p);if err!=nil||u.Scheme!="s3"||u.Host==""{return "","",false};k,err:=url.PathUnescape(strings.TrimPrefix(u.EscapedPath(),"/"));return u.Host,k,err==nil}
func Resolve(p string)(string,error){station,key,ok:=DecodePath(p);if !ok{return p,nil};c,exists:=ClientFor(station);if !exists{return "",errors.New("S3 station not configured")};return c.SignedURL(key,time.Hour*3)}
func New(cfg Config)(*Client,error){
 u,err:=url.Parse(cfg.Endpoint)
 if err!=nil||u.Scheme!="https"||u.Host==""||u.User!=nil||u.RawQuery!=""||u.Fragment!=""{return nil,errors.New("S3 endpoint must be an HTTPS base URL")}
 if cfg.Bucket==""||cfg.AccessKey==""||cfg.SecretKey==""{return nil,errors.New("S3 bucket and keys required")}
 if cfg.Region==""{cfg.Region="us-east-1"}
 if strings.Contains(cfg.Bucket,"/")||strings.Contains(cfg.Bucket,".."){return nil,errors.New("invalid bucket")}
 return &Client{cfg:cfg,base:u,http:&http.Client{Timeout:20*time.Second}},nil
}
func (c *Client) objectURL(key string)*url.URL{
 u:=*c.base
 parts:=[]string{strings.TrimRight(u.EscapedPath(),"/"),url.PathEscape(c.cfg.Bucket)}
 for _,part:=range strings.Split(key,"/"){parts=append(parts,url.PathEscape(part))}
 u.RawPath=strings.Join(parts,"/")
 u.Path,_=url.PathUnescape(u.RawPath)
 return &u
}
func (c *Client) bucketURL()*url.URL{
 u:=*c.base
 u.RawPath=strings.TrimRight(u.EscapedPath(),"/")+"/"+url.PathEscape(c.cfg.Bucket)
 u.Path,_=url.PathUnescape(u.RawPath)
 return &u
}
func hmac256(key []byte,msg string)[]byte{m:=hmac.New(sha256.New,key);m.Write([]byte(msg));return m.Sum(nil)}
func sumHex(b []byte)string{s:=sha256.Sum256(b);return hex.EncodeToString(s[:])}
func (c *Client) signingKey(date string)[]byte{k:=hmac256([]byte("AWS4"+c.cfg.SecretKey),date);k=hmac256(k,c.cfg.Region);k=hmac256(k,"s3");return hmac256(k,"aws4_request")}
func canonQuery(v url.Values)string{q:=v.Encode();return strings.ReplaceAll(q,"+","%20")}
func (c *Client) sign(method string,u *url.URL,bodyHash string,t time.Time,presign bool,expiry int)string{
 stamp:=t.UTC().Format("20060102T150405Z");day:=t.UTC().Format("20060102")
 scope:=day+"/"+c.cfg.Region+"/s3/aws4_request"
 credential:=c.cfg.AccessKey+"/"+scope
 values:=u.Query()
 if presign {
 values.Set("X-Amz-Algorithm","AWS4-HMAC-SHA256")
 values.Set("X-Amz-Credential",credential)
 values.Set("X-Amz-Date",stamp)
 values.Set("X-Amz-Expires",fmt.Sprint(expiry))
 values.Set("X-Amz-SignedHeaders","host")
 }
 u.RawQuery=canonQuery(values)
 headers:="host:"+u.Host+"\n"
 signed:="host"
 if !presign{headers+="x-amz-content-sha256:"+bodyHash+"\n"+"x-amz-date:"+stamp+"\n";signed+=";x-amz-content-sha256;x-amz-date"}
 canonical:=method+"\n"+u.EscapedPath()+"\n"+u.RawQuery+"\n"+headers+"\n"+signed+"\n"+bodyHash
 toSign:="AWS4-HMAC-SHA256\n"+stamp+"\n"+scope+"\n"+sumHex([]byte(canonical))
 signature:=hex.EncodeToString(hmac256(c.signingKey(day),toSign))
 if presign{values.Set("X-Amz-Signature",signature);u.RawQuery=canonQuery(values);return ""}
 return "AWS4-HMAC-SHA256 Credential="+credential+", SignedHeaders="+signed+", Signature="+signature
}
func(c *Client) SignedURL(key string,ttl time.Duration)(string,error){
 if key==""||ttl<time.Second||ttl>7*24*time.Hour{return "",errors.New("invalid object or URL lifetime")}
 u:=c.objectURL(key);c.sign("GET",u,"UNSIGNED-PAYLOAD",time.Now(),true,int(ttl.Seconds()));return u.String(),nil
}
func(c *Client) request(ctx context.Context,method string,u *url.URL,body io.Reader,size int64)(*http.Response,error){
 if body==nil{body=http.NoBody}
 r,err:=http.NewRequestWithContext(ctx,method,u.String(),body);if err!=nil{return nil,err}
 if size>=0{r.ContentLength=size}
 now:=time.Now().UTC();hash:="UNSIGNED-PAYLOAD"
 r.Header.Set("x-amz-date",now.Format("20060102T150405Z"));r.Header.Set("x-amz-content-sha256",hash)
 r.Header.Set("Authorization",c.sign(method,u,hash,now,false,0))
 return c.http.Do(r)
}
func(c *Client) List(ctx context.Context)([]Object,error){
 type listing struct { Contents []struct{Key string `xml:"Key"`;Size int64 `xml:"Size"`} `xml:"Contents"`;Next string `xml:"NextContinuationToken"`;Truncated bool `xml:"IsTruncated"` }
 var found []Object;continuation:=""
 for i:=0;i<100;i++{
 u:=c.bucketURL();q:=u.Query();q.Set("list-type","2");q.Set("max-keys","1000");if c.cfg.Prefix!=""{q.Set("prefix",c.cfg.Prefix)};if continuation!=""{q.Set("continuation-token",continuation)};u.RawQuery=canonQuery(q)
 resp,err:=c.request(ctx,"GET",u,nil,-1);if err!=nil{return nil,err}
 if resp.StatusCode!=200{resp.Body.Close();return nil,fmt.Errorf("S3 ListObjects failed: %d",resp.StatusCode)}
 var data listing;err=xml.NewDecoder(io.LimitReader(resp.Body,8<<20)).Decode(&data);resp.Body.Close();if err!=nil{return nil,err}
 for _,o:=range data.Contents{ext:=strings.ToLower(path.Ext(o.Key));switch ext{case ".mp3",".wav",".flac",".ogg",".opus",".m4a",".aac",".webm",".wma":found=append(found,Object{Key:o.Key,Size:o.Size})}}
 if !data.Truncated{sort.Slice(found,func(i,j int)bool{return found[i].Key<found[j].Key});return found,nil}
 if data.Next==""||data.Next==continuation{return nil,errors.New("invalid S3 continuation token")}
 continuation=data.Next
 }
 return nil,errors.New("too many S3 listing pages")
}

func (c *Client) Put(ctx context.Context,key string,body io.Reader,size int64) error {
 if key==""||size<0 {return errors.New("invalid S3 upload")}
 r,err:=c.request(ctx,"PUT",c.objectURL(key),body,size)
 if err!=nil{return err};defer r.Body.Close()
 if r.StatusCode<200||r.StatusCode>=300{return fmt.Errorf("S3 PUT failed: %d",r.StatusCode)}
 return nil
}
func (c *Client) Delete(ctx context.Context,key string) error {
 if key=="" {return errors.New("invalid S3 object key")}
 r,err:=c.request(ctx,"DELETE",c.objectURL(key),nil,-1)
 if err!=nil{return err};defer r.Body.Close()
 if r.StatusCode<200||r.StatusCode>=300{return fmt.Errorf("S3 DELETE failed: %d",r.StatusCode)}
 return nil
}
