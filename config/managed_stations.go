package config

import (
 "encoding/json"
 "errors"
 "os"
 "path/filepath"
 "strings"
 "sync"
)

type StationOverrides struct {
 Stations map[string]StationConfig `json:"stations"`
 Deleted []string `json:"deleted"`
}
var stationFileMu sync.Mutex

func StationFile() string {
 if p:=os.Getenv("CLIAMP_STATIONS_FILE");p!=""{return p}
 return "/data/stations.json"
}
func readOverrides()(*StationOverrides,error){
 data,err:=os.ReadFile(StationFile())
 if errors.Is(err,os.ErrNotExist){return &StationOverrides{Stations:map[string]StationConfig{}},nil}
 if err!=nil{return nil,err}
 var result StationOverrides
 if err:=json.Unmarshal(data,&result);err!=nil{return nil,err}
 if result.Stations==nil{result.Stations=map[string]StationConfig{}}
 return &result,nil
}
func (c *Config) LoadStationOverrides()error{
 overrides,err:=readOverrides();if err!=nil{return err}
 for _,id:=range overrides.Deleted{delete(c.Stations,id)}
 for id,station:=range overrides.Stations{if !validStationID(id){continue};c.Stations[id]=station}
 order:=make([]string,0,len(c.Stations));seen:=map[string]bool{}
 for _,id:=range c.StationOrder{if _,ok:=c.Stations[id];ok&&!seen[id]{order=append(order,id);seen[id]=true}}
 for id:=range c.Stations{if !seen[id]{order=append(order,id)}}
 c.StationOrder=order
 return nil
}
func validStationID(id string)bool{
 if id==""||len(id)>64||strings.HasPrefix(id,"."){return false}
 for _,r:=range id{if !(r>='a'&&r<='z'||r>='0'&&r<='9'||r=='-'){return false}}
 return true
}
func saveOverrides(data *StationOverrides)error{
 path:=StationFile()
 if err:=os.MkdirAll(filepath.Dir(path),0700);err!=nil{return err}
 tmp,err:=os.CreateTemp(filepath.Dir(path),".stations-*");if err!=nil{return err}
 defer os.Remove(tmp.Name())
 if err:=tmp.Chmod(0600);err!=nil{tmp.Close();return err}
 if err:=json.NewEncoder(tmp).Encode(data);err!=nil{tmp.Close();return err}
 if err:=tmp.Sync();err!=nil{tmp.Close();return err}
 if err:=tmp.Close();err!=nil{return err}
 return os.Rename(tmp.Name(),path)
}
// CreateManagedStation saves a station definition; it takes effect on restart.
func CreateManagedStation(id string,station StationConfig)error{
 if !validStationID(id){return errors.New("invalid station ID")}
 stationFileMu.Lock();defer stationFileMu.Unlock()
 data,err:=readOverrides();if err!=nil{return err}
 data.Stations[id]=station
 kept:=data.Deleted[:0];for _,v:=range data.Deleted{if v!=id{kept=append(kept,v)}};data.Deleted=kept
 return saveOverrides(data)
}
// DeleteManagedStation removes a station definition without deleting its music.
func DeleteManagedStation(id string)error{
 if !validStationID(id){return errors.New("invalid station ID")}
 stationFileMu.Lock();defer stationFileMu.Unlock()
 data,err:=readOverrides();if err!=nil{return err}
 delete(data.Stations,id)
 for _,existing:=range data.Deleted{if existing==id{return saveOverrides(data)}}
 data.Deleted=append(data.Deleted,id)
 return saveOverrides(data)
}

 // UpdateManagedStation persists an edit to a station defined in TOML or the managed overlay.
func UpdateManagedStation(id string,station StationConfig)error{
 if !validStationID(id){return errors.New("invalid station ID")}
 stationFileMu.Lock();defer stationFileMu.Unlock()
 data,err:=readOverrides();if err!=nil{return err}
 for _,deleted:=range data.Deleted{if deleted==id{return errors.New("station is marked for deletion")}}
 data.Stations[id]=station
 return saveOverrides(data)
}
