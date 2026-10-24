package geo

import (
 "encoding/json"
 "io"
 "errors"
 "fmt"
 "net"
 "net/http"
 "net/url"
 "strings"
 "strconv"
 "sync"
 "time"
 "github.com/oschwald/geoip2-golang"
)

type Location struct {
 Country string
 CountryCode string
 City string
 Latitude float64
 Longitude float64
}
type cached struct { location Location; expires time.Time }
type DB struct {
 reader *geoip2.Reader
 apiURL string
 options APIOptions
 client *http.Client
 mu sync.Mutex
 cache map[string]cached
}
func Open(path string)(*DB,error) {
 reader,err:=geoip2.Open(path);if err!=nil{return nil,err}
 return &DB{reader:reader,cache:map[string]cached{}},nil
}
type APIOptions struct { Param string; IPField string; CountryField string; CodeField string; CityField string; LatitudeField string; LongitudeField string; ValidField string; PrivateField string }
func OpenWithAPI(path,endpoint string)(*DB,error){return OpenWithProvider(path,endpoint,APIOptions{})}
func OpenWithProvider(path,endpoint string,opts APIOptions)(*DB,error){
 var db *DB
 var err error
 if path!="" {db,err=Open(path);if err!=nil{return nil,err}} else {db=&DB{cache:map[string]cached{}}}
 if endpoint!="" {
  u,err:=url.Parse(endpoint);if err!=nil||u.Scheme!="https"||u.Host==""||u.User!=nil||u.Fragment!="" {return nil,errors.New("geo API URL must be a valid HTTPS endpoint")}
  db.apiURL=endpoint;db.options=opts;db.client=&http.Client{Timeout:2*time.Second}
 }
 return db,nil
}
func (db *DB) Lookup(ipStr string) Location {
 ip:=net.ParseIP(ipStr)
 if ip==nil||ip.IsPrivate()||ip.IsLoopback()||ip.IsLinkLocalUnicast()||ip.IsMulticast()||ip.IsUnspecified(){return Location{}}
 if db.apiURL!="" {
  db.mu.Lock()
  if v,ok:=db.cache[ipStr];ok&&time.Now().Before(v.expires){db.mu.Unlock();return v.location}
  db.mu.Unlock()
  loc,err:=db.lookupAPI(ipStr)
  if err==nil&&loc.Country!="" {
   db.mu.Lock();if len(db.cache)>10000 {db.cache=map[string]cached{}};db.cache[ipStr]=cached{loc,time.Now().Add(24*time.Hour)};db.mu.Unlock()
   return loc
  }
 }
 if db.reader==nil{return Location{}}
 record,err:=db.reader.City(ip);if err!=nil{return Location{}}
 return Location{record.Country.Names["en"],record.Country.IsoCode,record.City.Names["en"],record.Location.Latitude,record.Location.Longitude}
}

func field(data any,path string) any {
 if path=="" {return nil}
 var result=data
 for _,part:=range strings.Split(path,".") {
  m,ok:=result.(map[string]any);if !ok{return nil}
  result=m[part]
 }
 return result
}
func str(data any,path string)string{v:=field(data,path);if v==nil{return ""};return strings.TrimSpace(fmt.Sprint(v))}
func float(data any,path string)float64{v:=field(data,path);if v==nil{return 0};n,_:=strconv.ParseFloat(fmt.Sprint(v),64);return n}
func(db *DB) lookupAPI(ip string)(Location,error){
 u,err:=url.Parse(db.apiURL);if err!=nil{return Location{},err}
 param:=db.options.Param
 if param=="" {param="ip"}
 q:=u.Query();q.Set(param,ip);u.RawQuery=q.Encode()
 req,err:=http.NewRequest(http.MethodGet,u.String(),nil);if err!=nil{return Location{},err}
 req.Header.Set("Accept","application/json")
 resp,err:=db.client.Do(req);if err!=nil{return Location{},err};defer resp.Body.Close()
 if resp.StatusCode!=200{return Location{},errors.New("geo provider returned non-200 status")}
 var payload any
 if err:=json.NewDecoder(io.LimitReader(resp.Body,64<<10)).Decode(&payload);err!=nil{return Location{},err}
 opts:=db.options
 defaults:=func(value,fallback string)string{if value!=""{return value};return fallback}
 if opts.IPField!=""&&str(payload,opts.IPField)!=ip{return Location{},errors.New("provider returned another IP")}
 if opts.ValidField!=""&&field(payload,opts.ValidField)==false{return Location{},errors.New("provider rejected IP")}
 if opts.PrivateField!=""&&field(payload,opts.PrivateField)==true{return Location{},errors.New("provider classified private IP")}
 loc:=Location{
 Country:str(payload,defaults(opts.CountryField,"country")),
 CountryCode:str(payload,defaults(opts.CodeField,"countryTag")),
 City:str(payload,defaults(opts.CityField,"city")),
 Latitude:float(payload,defaults(opts.LatitudeField,"latitude")),
 Longitude:float(payload,defaults(opts.LongitudeField,"longitude")),
 }
 if loc.Country=="" {return Location{},errors.New("country missing")}
 return loc,nil
}
func(db *DB) Close()error{if db.reader!=nil{return db.reader.Close()};return nil}
