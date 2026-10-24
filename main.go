package main

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"cliamp-server/broadcast"
 "path/filepath"
	"cliamp-server/config"
	"cliamp-server/geo"
	"cliamp-server/library"
	"cliamp-server/playlist"
	"cliamp-server/scheduler"
	"cliamp-server/server"
	"cliamp-server/stats"
 "cliamp-server/s3store"
 "strings"
)

// raiseFileLimit raises the soft file-descriptor limit to the hard limit.
// Each connected listener holds one socket, so the default soft limit of
// 1024 caps the server at roughly 1000 simultaneous listeners.
func raiseFileLimit() {
	var rlimit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rlimit); err != nil {
		return
	}
	if rlimit.Cur < rlimit.Max {
		prev := rlimit.Cur
		rlimit.Cur = rlimit.Max
		if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rlimit); err == nil {
			slog.Info("raised file descriptor limit", "from", prev, "to", rlimit.Max)
		}
	}
}

func main() {
	cfg, err := config.Load(config.ConfigPathFromArgs())
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	if config.ParseFlags(cfg) {
		return
	}

	// Set up structured logging
	var level slog.Level
	switch cfg.Log.Level {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

	raiseFileLimit()

	// Validate config
	if err := cfg.Validate(); err != nil {
		slog.Error("invalid config", "error", err)
		os.Exit(1)
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		slog.Error("ffmpeg is required", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Load persisted IP bans before accepting radio connections.
 banFile:=os.Getenv("CLIAMP_BAN_FILE")
 if banFile=="" {banFile=filepath.Join(os.TempDir(),"cliamp-banned-ips.json")}
 if err:=broadcast.InitBans(banFile);err!=nil {slog.Error("failed to load listener bans","error",err);os.Exit(1)}

 // Build stations
	stations := make(map[string]*server.Station, len(cfg.Stations))

	for id, stCfg := range cfg.Stations {
		slog.Info("scanning library", "station", id, "path", stCfg.Path, "recursive", stCfg.Recursive)

		var tracks []library.Track
  var err error
  if len(stCfg.RemoteURLs)>0 {for _,remoteURL:=range stCfg.RemoteURLs{u:=strings.Split(remoteURL,"?")[0];name:=filepath.Base(u);tracks=append(tracks,library.Track{Path:remoteURL,Title:strings.TrimSuffix(name,filepath.Ext(name))})}}else if stCfg.S3.Endpoint!="" {
   remote:=stCfg.S3
   clientErr:=s3store.Register(id,s3store.Config{Endpoint:remote.Endpoint,Region:remote.Region,Bucket:remote.Bucket,Prefix:remote.Prefix,AccessKey:os.Getenv(remote.AccessKeyEnv),SecretKey:os.Getenv(remote.SecretKeyEnv)})
   if clientErr!=nil{slog.Error("S3 station failed", "station",id,"error",clientErr);os.Exit(1)}
   client,_:=s3store.ClientFor(id)
   objects,listErr:=client.List(ctx)
   if listErr!=nil{slog.Error("S3 music list failed","station",id,"error",listErr);os.Exit(1)}
   for _,obj:=range objects{base:=obj.Key;if j:=strings.LastIndex(base,"/");j>=0{base=base[j+1:]};title:=strings.TrimSuffix(base,filepath.Ext(base));tracks=append(tracks,library.Track{Path:s3store.TrackPath(id,obj.Key),Title:title})}
  }else{tracks,err=library.Scan(stCfg.Path,stCfg.Recursive)}
		if err != nil {
			slog.Error("failed to scan library", "station", id, "error", err)
			os.Exit(1)
		}

		if err:=library.ApplyMetadata(id,tracks);err!=nil{slog.Error("failed to read track metadata overrides","station",id,"error",err);os.Exit(1)}
  // Exclude intro file from music rotation
		if stCfg.IntroFile != "" {
			filtered := tracks[:0]
			for _, t := range tracks {
				if t.Path != stCfg.IntroFile {
					filtered = append(filtered, t)
				}
			}
			tracks = filtered
		}

		if len(tracks) == 0 {
			slog.Error("no MP3 files found", "station", id, "path", stCfg.Path)
			os.Exit(1)
		}

		pl := playlist.New(tracks, stCfg.Shuffle)

		var source playlist.TrackSource = pl
 var mediaScheduler *scheduler.Scheduler

		if stCfg.AdsPath != "" || stCfg.JinglesPath != "" {
			sched, err := scheduler.New(pl, scheduler.Config{
				AdsPath:      stCfg.AdsPath,
				AdEverySongs: stCfg.AdEveryNSongs,
				AdEveryMins:  stCfg.AdEveryNMins,
				AdShuffle:    stCfg.AdShuffle,
 JinglesPath: stCfg.JinglesPath,
 JingleEverySongs: stCfg.JingleEveryNSongs,
 JingleShuffle: stCfg.JingleShuffle,
			})
			if err != nil {
				slog.Error("failed to create scheduler", "station", id, "error", err)
				os.Exit(1)
			}
			source = sched
 mediaScheduler = sched
			slog.Info("scheduler enabled", "station", id)
		}

		hub := broadcast.NewHub(id, source, cfg.Stream.BufferSize, cfg.Stream.MaxListeners)

		go hub.Run(ctx)

		stations[id] = &server.Station{
 ID: id,
			Hub:        hub,
			Config:     stCfg,
			TrackCount: pl.Len(),
 Playlist: pl,
 Scheduler: mediaScheduler,
			Tracks:     tracks,
		}
	}

	// Open optional GeoIP database
	var geoDB *geo.DB
	if cfg.Geo.DBPath != "" || cfg.Geo.APIURL != "" {
		var err error
		geoDB, err = geo.OpenWithProvider(cfg.Geo.DBPath,cfg.Geo.APIURL,geo.APIOptions{Param:cfg.Geo.APIParam,IPField:cfg.Geo.APIIPField,CountryField:cfg.Geo.APICountryField,CodeField:cfg.Geo.APICountryCodeField,CityField:cfg.Geo.APICityField,LatitudeField:cfg.Geo.APILatitudeField,LongitudeField:cfg.Geo.APILongitudeField,ValidField:cfg.Geo.APIValidField,PrivateField:cfg.Geo.APIPrivateField})
		if err != nil {
			slog.Error("failed to open geo database", "path", cfg.Geo.DBPath, "error", err)
			os.Exit(1)
		}
		defer geoDB.Close()
		slog.Info("geo location provider configured", "local_db", cfg.Geo.DBPath != "", "remote_api", cfg.Geo.APIURL != "")
	}

	// Open optional statistics database
	var statsDB *stats.DB
	if cfg.Stats.DBPath != "" {
		var err error
		statsDB, err = stats.Open(cfg.Stats.DBPath)
		if err != nil {
			slog.Error("failed to open stats database", "path", cfg.Stats.DBPath, "error", err)
			os.Exit(1)
		}
		defer statsDB.Close()
		slog.Info("statistics database loaded", "path", cfg.Stats.DBPath)

		stationIDs := make([]string, 0, len(stations))
		for id := range stations {
			stationIDs = append(stationIDs, id)
		}
		peakTracker, err := stats.NewPeakTracker(statsDB, stationIDs)
		if err != nil {
			slog.Error("failed to initialize listener peak tracking", "error", err)
			os.Exit(1)
		}

		// Minimum session duration to filter out media-player probe connections
		// that connect briefly to check stream metadata before the real playback.
		minSession := int64(cfg.Stats.MinSession)
		if minSession <= 0 {
			minSession = 10
		}

		// Wire disconnect hooks to record sessions
		for _, st := range stations {
			st.Hub.SetListenerChangeHook(func(stationID string, delta int) {
				if err := peakTracker.Change(stationID, delta); err != nil {
					slog.Error("failed to record listener peak", "station", stationID, "error", err)
				}
			})
			st.Hub.SetDisconnectHook(func(stationID string, snap broadcast.ListenerSnapshot, disconnectedAt time.Time) {
				dur := int64(disconnectedAt.Sub(snap.ConnectedAt).Seconds())
				if dur < minSession {
					slog.Debug("session too short, not recording", "station", stationID, "duration", dur, "min", minSession)
					return
				}
				if err := statsDB.Record(stats.Session{
					Station:         stationID,
					Country:         snap.Info.Country,
					CountryCode:     snap.Info.CountryCode,
					City:            snap.Info.City,
					Latitude:        snap.Info.Latitude,
					Longitude:       snap.Info.Longitude,
					ConnectedAt:     snap.ConnectedAt,
					DisconnectedAt:  disconnectedAt,
					DurationSeconds: dur,
				}); err != nil {
					slog.Error("failed to record listener session", "station", stationID, "error", err)
				}
			})
		}
	} else {
		slog.Info("statistics disabled (no --stats-db configured)")
	}

	if cfg.Admin.Password != "" {
		slog.Info("admin password protection enabled")
	} else {
		slog.Warn("admin password not set, status endpoints are public")
	}

	// Create and start HTTP server
	srv := server.New(cfg, stations, geoDB, statsDB)

	// Graceful shutdown on SIGINT/SIGTERM
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigCh
		slog.Info("received signal, shutting down", "signal", sig)
		cancel()
		srv.Shutdown(context.Background())
	}()

	if err := srv.ListenAndServe(); err != nil && err.Error() != "http: Server closed" {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}
