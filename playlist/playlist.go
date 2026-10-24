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
 p.buildOrder()
 return nil
}

