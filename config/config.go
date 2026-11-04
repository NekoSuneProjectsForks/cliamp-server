package config

import (
	"fmt"
	"os"
	"path/filepath"
 "net/url"

	"github.com/BurntSushi/toml"
)

const minBufferSizeKB = 64

type ServerConfig struct {
	Host string `toml:"host"`
	Port int    `toml:"port"`
}

type StationConfig struct {
	Name        string `toml:"name"`
	Description string `toml:"description"`
	Genre       string `toml:"genre"`
	URL         string `toml:"url"`
	Path        string `toml:"path"`
 S3 S3Config `toml:"s3"`
 RemoteURLs []string `toml:"remote_urls" json:"remote_urls"`
 YouTubeURLs []string `toml:"youtube_urls" json:"youtube_urls"`
 Source string `toml:"source" json:"source"`
 Relays []RelayConfig `toml:"relays" json:"relays"`
	Shuffle     bool   `toml:"shuffle"`
	Recursive   bool   `toml:"recursive"`

	// ExposeTracks publishes the station's library as individual files at
	// /{station}/tracks. Off by default: streaming a library and offering it
	// as downloads are different things, so an operator must opt in per station.
	ExposeTracks bool `toml:"expose_tracks"`

	// Intro: single audio file played once when a listener connects
	IntroFile string `toml:"intro_file"`

	// Ads: audio files injected between songs
	AdsPath       string `toml:"ads_path"`
	AdEveryNSongs int    `toml:"ad_every_n_songs"`
	AdEveryNMins  int    `toml:"ad_every_n_minutes"`
	AdShuffle     bool   `toml:"ad_shuffle"`
 JinglesPath string `toml:"jingles_path"`
 JingleEveryNSongs int `toml:"jingle_every_n_songs"`
 JingleShuffle bool `toml:"jingle_shuffle"`
}

type RelayConfig struct {
 Type string `toml:"type" json:"type"`
 Name string `toml:"name" json:"name"`
 Host string `toml:"host" json:"host"`
 Port int `toml:"port" json:"port"`
 Mount string `toml:"mount" json:"mount"`
 PasswordEnv string `toml:"password_env" json:"-"`
 Public bool `toml:"public" json:"public"`
 TLS bool `toml:"tls" json:"tls"`
}

type S3Config struct {
 Endpoint string `toml:"endpoint" json:"endpoint"`
 Region string `toml:"region" json:"region"`
 Bucket string `toml:"bucket" json:"bucket"`
 Prefix string `toml:"prefix" json:"prefix"`
 AccessKeyEnv string `toml:"access_key_env" json:"access_key_env"`
 SecretKeyEnv string `toml:"secret_key_env" json:"secret_key_env"`
}

type StreamConfig struct {
	MetaInt      int `toml:"metaint"`
	BufferSize   int `toml:"buffer_size"`
	MaxListeners int `toml:"max_listeners"`  // 0 = unlimited
	IntroSeenTTL int `toml:"intro_seen_ttl"` // Hours before intro replays for same IP (default 48)
}

type AdminConfig struct {
	Password string `toml:"password"`
}

type LogConfig struct {
	Level string `toml:"level"`
}

type GeoConfig struct {
	APIURL string `toml:"api_url"`
 APIParam string `toml:"api_param"`
 APIIPField string `toml:"api_ip_field"`
 APICountryField string `toml:"api_country_field"`
 APICountryCodeField string `toml:"api_country_code_field"`
 APICityField string `toml:"api_city_field"`
 APILatitudeField string `toml:"api_latitude_field"`
 APILongitudeField string `toml:"api_longitude_field"`
 APIValidField string `toml:"api_valid_field"`
 APIPrivateField string `toml:"api_private_field"`
	DBPath string `toml:"db_path"`
}

type StatsConfig struct {
	DBPath     string `toml:"db_path"`
	MinSession int    `toml:"min_session"` // Minimum session duration in seconds to record (default 10)
}

type Config struct {
	Server   ServerConfig             `toml:"server"`
	Stream   StreamConfig             `toml:"stream"`
	Admin    AdminConfig              `toml:"admin"`
	Log      LogConfig                `toml:"log"`
	Geo      GeoConfig                `toml:"geo"`
	Stats    StatsConfig              `toml:"stats"`
	Stations map[string]StationConfig `toml:"stations"`

	// StationOrder holds station IDs in the order they appear in the config file.
	StationOrder []string `toml:"-"`
}

func Defaults() *Config {
    statsPath := ""
    if info, err := os.Stat("/data"); err == nil && info.IsDir() { statsPath = "/data/stats.db" }
	return &Config{
		Server: ServerConfig{
			Host: "0.0.0.0",
			Port: 8000,
		},
		Stream: StreamConfig{
			MetaInt:    8192,
			BufferSize: 512,
		},
		Log: LogConfig{
			Level: "info",
		},
        Stats: StatsConfig{DBPath: statsPath, MinSession: 10},
	}
}

// Load reads the config file. If configPath is non-empty, it reads that file
// directly. Otherwise it falls back to ~/.config/cliamp-server/config.toml.
func Load(configPath string) (*Config, error) {
	cfg := Defaults()

	path := configPath
	if path == "" {
		configDir, err := os.UserConfigDir()
		if err != nil {
			return cfg, nil
		}
		path = filepath.Join(configDir, "cliamp-server", "config.toml")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) && configPath == "" {
			return cfg, nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	meta, err := toml.Decode(string(data), cfg)
	if err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	// Extract station IDs in config-file order from the TOML metadata.
	for _, key := range meta.Keys() {
		if len(key) == 2 && key[0] == "stations" {
			cfg.StationOrder = append(cfg.StationOrder, key[1])
		}
	}

	if err:=cfg.LoadStationOverrides();err!=nil{return nil,fmt.Errorf("loading managed stations: %w",err)}
 return cfg,nil
}

func (c *Config) Validate() error {
	if len(c.Stations) == 0 {
		return fmt.Errorf("at least one station is required (use --music or define [stations.*] in config)")
	}

	for id, st := range c.Stations {
		for _, relay := range st.Relays {
            if relay.Type != "icecast2" {
                return fmt.Errorf("station %q: relay type %q not supported (icecast2 only)", id, relay.Type)
            }
            if relay.Host == "" || relay.Port < 1 || relay.Port > 65535 || relay.Mount == "" || relay.Mount[0] != '/' || relay.PasswordEnv == "" {
                return fmt.Errorf("station %q: relay requires host, valid port, /mount and password_env", id)
            }
            if os.Getenv(relay.PasswordEnv) == "" {
                return fmt.Errorf("station %q: relay password environment variable %q is not set", id, relay.PasswordEnv)
            }
        }
		if st.Source=="youtube" {
   for _,raw:=range st.YouTubeURLs {if !ValidYouTubeURL(raw){return fmt.Errorf("station %q: invalid YouTube video URL",id)}}
  } else if len(st.RemoteURLs)>0 {
   for _,raw:=range st.RemoteURLs{u,e:=url.Parse(raw);if e!=nil||u.Scheme!="https"||u.Host==""||u.User!=nil{return fmt.Errorf("station %q: remote_urls require HTTPS absolute URLs",id)}}
  }else if st.S3.Endpoint!="" {
   if st.S3.Bucket==""||st.S3.AccessKeyEnv==""||st.S3.SecretKeyEnv=="" {return fmt.Errorf("station %q: S3 bucket and credential env variable names required",id)}
   if _,err:=url.ParseRequestURI(st.S3.Endpoint);err!=nil{return fmt.Errorf("station %q: invalid S3 URL: %w",id,err)}
  }else if st.Path == "" {
			return fmt.Errorf("station %q: library path is required", id)
		}

		if st.Source!="youtube" && st.S3.Endpoint=="" && len(st.RemoteURLs)==0 {
  info, err := os.Stat(st.Path)
		if err != nil {
			return fmt.Errorf("station %q: library path %q: %w", id, st.Path, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("station %q: library path %q is not a directory", id, st.Path)
		}
  }

		// Validate intro file
		if st.IntroFile != "" {
			fi, err := os.Stat(st.IntroFile)
			if err != nil {
				return fmt.Errorf("station %q: intro_file %q: %w", id, st.IntroFile, err)
			}
			if fi.IsDir() {
				return fmt.Errorf("station %q: intro_file %q must be a file, not a directory", id, st.IntroFile)
			}
		}

		// Validate ads config
		if st.AdsPath != "" {
			if _, err := os.Stat(st.AdsPath); err != nil {
				return fmt.Errorf("station %q: ads_path %q: %w", id, st.AdsPath, err)
			}
			if st.AdEveryNSongs <= 0 && st.AdEveryNMins <= 0 {
				return fmt.Errorf("station %q: ads_path is set but neither ad_every_n_songs nor ad_every_n_minutes is > 0", id)
			}
		}
		if st.JinglesPath != "" {
   if info,err:=os.Stat(st.JinglesPath);err!=nil{return fmt.Errorf("station %q jingles_path: %w",id,err)}else if !info.IsDir(){return fmt.Errorf("station %q jingles_path must be a directory",id)}
   if st.JingleEveryNSongs<=0{return fmt.Errorf("station %q needs jingle_every_n_songs > 0",id)}
  }
 }

 if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid port: %d", c.Server.Port)
	}

	if c.Stream.MetaInt < 256 {
		return fmt.Errorf("metaint must be at least 256 bytes")
	}

	if c.Stream.BufferSize < minBufferSizeKB {
		return fmt.Errorf("buffer_size must be at least %d KB", minBufferSizeKB)
	}

	if c.Geo.APIURL != "" {
		u,err:=url.Parse(c.Geo.APIURL)
		if err!=nil || u.Scheme!="https" || u.Host=="" || u.User!=nil {return fmt.Errorf("geo api_url must be a valid HTTPS endpoint")}
	}
	if c.Geo.DBPath != "" {
		if _, err := os.Stat(c.Geo.DBPath); err != nil {
			return fmt.Errorf("geo db_path %q: %w", c.Geo.DBPath, err)
		}
	}

	if c.Stats.DBPath != "" {
		dir := filepath.Dir(c.Stats.DBPath)
		if _, err := os.Stat(dir); err != nil {
			return fmt.Errorf("stats db_path parent directory %q: %w", dir, err)
		}
	}

	return nil
}

// validYouTubeURL limits the extractor to recognized YouTube links and
// requires a video ID, rather than accepting arbitrary URLs as FFmpeg input.
func ValidYouTubeURL(raw string) bool {
 u,err:=url.Parse(raw);if err!=nil||u.Scheme!="https"||u.User!=nil||u.Port()!="" {return false}
 host:=u.Hostname()
 switch host {
 case "youtube.com","www.youtube.com","m.youtube.com","music.youtube.com":
  if u.Path!="/watch" {return false}
  id:=u.Query().Get("v");return validVideoID(id)
 case "youtu.be","www.youtu.be":
  return validVideoID(filepath.Base(u.Path)) && u.Path=="/"+filepath.Base(u.Path)
 default:return false
 }
}
func validVideoID(id string) bool {
 if len(id)!=11{return false}
 for _,c:=range id {if !((c>='a'&&c<='z')||(c>='A'&&c<='Z')||(c>='0'&&c<='9')||c=='_'||c=='-'){return false}}
 return true
}
