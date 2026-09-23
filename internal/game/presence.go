package game

import "sync"

// Who is connected. Used by HTTP handlers, so it has its own lock instead of
// living on the world goroutine.
type presence struct {
	mu     sync.Mutex
	online map[int]bool
}

// Marks a player online. Returns false if they already were.
func (p *presence) ClaimOnline(id int) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.online[id] {
		return false
	}
	p.online[id] = true
	return true
}

func (p *presence) ReleaseOnline(id int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.online, id)
}

func (p *presence) IsOnline(id int) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.online[id]
}

func (p *presence) Count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.online)
}
