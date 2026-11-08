package playlist

import (
	"math/rand/v2"
	"sync"
 "errors"

	"cliamp-server/library"
)

type Playlist struct {
	mu      sync.Mutex
	tracks  []library.Track
	order   []int
	pos     int
	shuffle bool
 revision uint64
}

func New(tracks []library.Track, shuffle bool) *Playlist {
	p := &Playlist{
		tracks:  tracks,
		shuffle: shuffle,
	}
	p.buildOrder()
	return p
}

func (p *Playlist) buildOrder() {
	p.order = make([]int, len(p.tracks))
	for i := range p.order {
		p.order[i] = i
	}
	if p.shuffle {
		rand.Shuffle(len(p.order), func(i, j int) {
			p.order[i], p.order[j] = p.order[j], p.order[i]
		})
	}
	p.pos = 0
}

func (p *Playlist) Next() library.Track {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.order)==0 {return library.Track{}}
 track := p.tracks[p.order[p.pos]]

	p.pos++
	if p.pos >= len(p.order) {
		p.buildOrder()
	}

	return track
}

func (p *Playlist) Current() library.Track {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.order) == 0 {
		return library.Track{}
	}
	return p.tracks[p.order[p.pos]]
}

func (p *Playlist) Len() int {
 p.mu.Lock();defer p.mu.Unlock();return len(p.tracks)
}

// Replace updates the upcoming rotation, keeping the current encoder and listeners alive.
// Reject an empty library so there is always an audio source for the next transition.
func (p *Playlist) Replace(tracks []library.Track) error {
 if len(tracks)==0{return errors.New("station library cannot be empty")}
 p.mu.Lock();defer p.mu.Unlock()
 p.tracks=append([]library.Track(nil),tracks...)
 p.revision++
 p.buildOrder()
 return nil
}


// HasTrack reports whether a preloaded source is still in the active
// rotation. The lock makes it safe to call while the dashboard replaces
// the playlist, and it never touches the current audio decoder.
func (p *Playlist) HasTrack(path string) bool {
 p.mu.Lock()
 defer p.mu.Unlock()
 for _,track:=range p.tracks {
  if track.Path==path{return true}
 }
 return false
}

// Revision changes on every successful Replace, so a caller can detect
// library edits even if the new playlist retains the same number of songs.
func (p *Playlist) Revision() uint64 {
 p.mu.Lock()
 defer p.mu.Unlock()
 return p.revision
}

// Snapshot returns current playlist tracks without advancing or reshuffling them.
// It lets the on-demand library API follow uploads and removals immediately.
func(p *Playlist) Snapshot() []library.Track {
 p.mu.Lock();defer p.mu.Unlock()
 return append([]library.Track(nil),p.tracks...)
}
