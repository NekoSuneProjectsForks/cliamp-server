package geo

import (
 "encoding/json"
 "io"
 "errors"
 "net"
 "net/http"
 "net/url"
 "strings"
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
 client *http.Client
 mu sync.Mutex
 cache map[string]cached
}
func Open(path string)(*DB,error) {
 reader,err:=geoip2.Open(path);if err!=nil{return nil,err}
 return &DB{reader:reader,cache:map[string]cached{}},nil
}
func OpenWithAPI(path,endpoint string)(*DB,error){
 var db *DB
 var err error
 if path!="" {db,err=Open(path);if err!=nil{return nil,err}} else {db=&DB{cache:map[string]cached{}}}
 if endpoint!="" {
  u,err:=url.Parse(endpoint);if err!=nil||u.Scheme!="https"||u.Host==""||u.User!=nil||u.Fragment!="" {return nil,errors.New("geo API URL must be a valid HTTPS endpoint")}
  db.apiURL=endpoint;db.client=&http.Client{Timeout:2*time.Second}
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
func(db *DB) lookupAPI(ip string)(Location,error){
 u,err:=url.Parse(db.apiURL);if err!=nil{return Location{},err}
 q:=u.Query();q.Set("ip",ip);u.RawQuery=q.Encode()
 req,err:=http.NewRequest(http.MethodGet,u.String(),nil);if err!=nil{return Location{},err}
 req.Header.Set("Accept","application/json")
 resp,err:=db.client.Do(req);if err!=nil{return Location{},err};defer resp.Body.Close()
 if resp.StatusCode!=200{return Location{},errors.New("geo provider returned non-200 status")}
 var payload struct {
  IP string `json:"ip"`
  Country string `json:"country"`
  CountryTag string `json:"countryTag"`
  City *string `json:"city"`
  Latitude float64 `json:"latitude"`
  Longitude float64 `json:"longitude"`
  Valid bool `json:"valid"`
  IsPrivate bool `json:"is_private"`
 }
 if err:=json.NewDecoder(io.LimitReader(resp.Body,64<<10)).Decode(&payload);err!=nil{return Location{},err}
 if !payload.Valid||payload.IsPrivate||!strings.EqualFold(payload.IP,ip){return Location{},errors.New("invalid geo API result")}
 city:="";if payload.City!=nil{city=*payload.City}
 return Location{payload.Country,payload.CountryTag,city,payload.Latitude,payload.Longitude},nil
}
func(db *DB) Close()error{if db.reader!=nil{return db.reader.Close()};return nil}
