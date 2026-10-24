package broadcast

import (
 "encoding/json"
 "errors"
 "net"
 "os"
 "path/filepath"
 "sort"
 "sync"
)

var bans = struct { sync.RWMutex; set map[string]bool; path string }{set:map[string]bool{}}

func InitBans(path string) error {
 bans.Lock(); defer bans.Unlock()
 bans.path=path
 data,err:=os.ReadFile(path)
 if errors.Is(err,os.ErrNotExist){return nil}
 if err!=nil{return err}
 var items []string
 if err:=json.Unmarshal(data,&items);err!=nil{return err}
 next:=make(map[string]bool)
 for _,ip:=range items { if net.ParseIP(ip)!=nil {next[ip]=true} }
 bans.set=next
 return nil
}
func IsBanned(ip string) bool { bans.RLock();defer bans.RUnlock();return bans.set[ip] }
func ListBans() []string {
 bans.RLock();defer bans.RUnlock()
 result:=make([]string,0,len(bans.set))
 for ip:=range bans.set {result=append(result,ip)}
 sort.Strings(result);return result
}
func SetBan(ip string,blocked bool) error {
 if net.ParseIP(ip)==nil{return errors.New("invalid IP address")}
 bans.Lock();defer bans.Unlock()
 next:=make(map[string]bool,len(bans.set))
 for k,v:=range bans.set{next[k]=v}
 if blocked {next[ip]=true}else{delete(next,ip)}
 values:=make([]string,0,len(next));for k:=range next{values=append(values,k)};sort.Strings(values)
 data,err:=json.MarshalIndent(values,"","  ");if err!=nil{return err}
 if bans.path=="" {return errors.New("ban persistence not initialized")}
 if err:=os.MkdirAll(filepath.Dir(bans.path),0700);err!=nil{return err}
 tmp,err:=os.CreateTemp(filepath.Dir(bans.path),".bans-*");if err!=nil{return err}
 defer os.Remove(tmp.Name())
 if err=tmp.Chmod(0600);err!=nil{tmp.Close();return err}
 if _,err=tmp.Write(data);err!=nil{tmp.Close();return err}
 if err=tmp.Close();err!=nil{return err}
 if err=os.Rename(tmp.Name(),bans.path);err!=nil{return err}
 bans.set=next;return nil
}
