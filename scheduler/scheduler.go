package scheduler

import (
	"time"
 "sync"

	"cliamp-server/library"
	"cliamp-server/playlist"
)

// Config holds the configuration for a Scheduler.
type Config struct {
	AdsPath      string // Path to ads directory or single file (empty = no ads)
	AdEverySongs int    // Play ad after every N music tracks (0 = disabled)
	AdEveryMins  int    // Play ad after every N minutes (0 = disabled)
	AdShuffle    bool   // Randomize ad selection
 JinglesPath string // Optional directory of station jingles
 JingleEverySongs int // Jingle after every N songs
 JingleShuffle bool
}

// Scheduler sits between the Hub and Playlist, injecting ad tracks.
// It implements playlist.TrackSource.
type Scheduler struct {
	music        playlist.TrackSource
	mu sync.Mutex
 adPool       *AdPool
 jinglePool *AdPool
 jingleEverySongs int
 songsSinceJingle int
	adEverySongs int
	adEveryMins  int
	songsSinceAd int
	lastAdTime   time.Time
}

// New creates a Scheduler wrapping the given music source.
// Returns an error if ad pool creation fails.
func New(music playlist.TrackSource, cfg Config) (*Scheduler, error) {
	s := &Scheduler{
		music:        music,
 jingleEverySongs: cfg.JingleEverySongs,
		adEverySongs: cfg.AdEverySongs,
		adEveryMins:  cfg.AdEveryMins,
		lastAdTime:   time.Now(),
	}

	if cfg.AdsPath != "" {
		pool, err := NewAdPool(cfg.AdsPath, cfg.AdShuffle)
		if err != nil {
			return nil, err
		}
		s.adPool = pool
	}

	if cfg.JinglesPath!="" {
  pool,err:=NewAdPool(cfg.JinglesPath,cfg.JingleShuffle)
  if err!=nil{return nil,err}
  s.jinglePool=pool
 }
 return s,nil
}

// Next returns the next track to play: ads (periodic) or music.
func (s *Scheduler) Next() library.Track {
 s.mu.Lock()
 defer s.mu.Unlock()
	// 1. Ads: check if a trigger has fired
	if s.adPool != nil && s.shouldPlayAd() {
		s.songsSinceAd = 0
		s.lastAdTime = time.Now()
		return s.adPool.Next()
	}

	// 2. Station jingles between songs, separate from paid ads.
 if s.jinglePool!=nil && s.jingleEverySongs>0 && s.songsSinceJingle>=s.jingleEverySongs {
  s.songsSinceJingle=0
  return s.jinglePool.Next()
 }
 // 3. Regular music
 s.songsSinceJingle++
 s.songsSinceAd++
	return s.music.Next()
}

// shouldPlayAd returns true if either the song count or time trigger has fired.
func (s *Scheduler) shouldPlayAd() bool {
	if s.adEverySongs > 0 && s.songsSinceAd >= s.adEverySongs {
		return true
	}
	if s.adEveryMins > 0 && time.Since(s.lastAdTime) >= time.Duration(s.adEveryMins)*time.Minute {
		return true
	}
	return false
}

// ReloadPools replaces scheduling content without recreating a running stream encoder.
func (s *Scheduler) ReloadPools(adPath string,adShuffle bool,jinglePath string,jingleShuffle bool) error {
 var ads,jingles *AdPool
 var err error
 if adPath!="" {ads,err=NewAdPool(adPath,adShuffle);if err!=nil{return err}}
 if jinglePath!="" {jingles,err=NewAdPool(jinglePath,jingleShuffle);if err!=nil{return err}}
 s.mu.Lock();defer s.mu.Unlock()
 s.adPool=ads
 s.jinglePool=jingles
 return nil
}
