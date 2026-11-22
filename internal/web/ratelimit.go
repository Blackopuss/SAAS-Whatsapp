package web

import (
	"sync"
	"time"
)

// limiter: ventana fija por llave (IP o email). En memoria: se reinicia al reiniciar el proceso.
// shortcut: un solo proceso; si algún día hay varias instancias, mover a Postgres.
type limiter struct {
	mu     sync.Mutex
	max    int
	window time.Duration
	hits   map[string]window
	now    func() time.Time
}

type window struct {
	count int
	reset time.Time
}

func newLimiter(max int, w time.Duration) *limiter {
	return &limiter{max: max, window: w, hits: map[string]window{}, now: time.Now}
}

// allow cuenta un intento y dice si sigue dentro del límite.
func (l *limiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if len(l.hits) > 10000 { // limpia ventanas vencidas para no crecer sin fin
		for k, w := range l.hits {
			if now.After(w.reset) {
				delete(l.hits, k)
			}
		}
	}
	w := l.hits[key]
	if now.After(w.reset) {
		w = window{reset: now.Add(l.window)}
	}
	w.count++
	l.hits[key] = w
	return w.count <= l.max
}
