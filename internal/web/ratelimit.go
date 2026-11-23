package web

import (
	"sync"
	"time"
)

// maxKeys: tope de llaves en memoria. Sin tope, quien mande miles de emails o IPs distintas
// haría crecer el mapa sin fin y cada intento tendría que barrerlo completo.
const maxKeys = 50000

// limiter: ventana fija por llave (IP o email). En memoria: se reinicia al reiniciar el proceso.
// shortcut: un solo proceso; si algún día hay varias instancias, mover a Postgres.
type limiter struct {
	mu     sync.Mutex
	max    int
	window time.Duration
	hits   map[string]window
	now    func() time.Time
	swept  time.Time
	// strict: lleno y sin ventanas vencidas, no saca llaves vivas; las nuevas cuentan como bloqueadas.
	strict bool
}

type window struct {
	count int
	reset time.Time
}

func newLimiter(max int, w time.Duration) *limiter {
	return &limiter{max: max, window: w, hits: map[string]window{}, now: time.Now}
}

// newStrictLimiter: para bloqueos por cuenta. Inundar de llaves no puede borrar el bloqueo de una víctima;
// a lo más deja sin entrar a cuentas nuevas sin dispositivo conocido mientras dure el ataque.
func newStrictLimiter(max int, w time.Duration) *limiter {
	l := newLimiter(max, w)
	l.strict = true
	return l
}

// allow cuenta un intento y dice si sigue dentro del límite.
func (l *limiter) allow(key string) bool { return l.count(key) <= l.max }

// count cuenta un intento y devuelve cuántos lleva la llave en su ventana.
func (l *limiter) count(key string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.hit(key, l.now()).count
}

// fail cuenta un intento fallido (para llaves que solo se castigan cuando fallan, como el email).
func (l *limiter) fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.hit(key, l.now())
}

// blocked dice si la llave ya llegó al límite, sin contar un intento más.
func (l *limiter) blocked(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	w, ok := l.hits[key]
	if !ok {
		return l.strict && !l.makeRoom(l.now())
	}
	return !l.now().After(w.reset) && w.count >= l.max
}

func (l *limiter) hit(key string, now time.Time) window {
	w, ok := l.hits[key]
	if !ok || now.After(w.reset) {
		if !ok && !l.makeRoom(now) {
			return window{count: l.max + 1, reset: now.Add(l.window)} // sin lugar (strict): cuenta como bloqueada
		}
		w = window{reset: now.Add(l.window)}
	}
	w.count++
	l.hits[key] = w
	return w
}

// makeRoom mantiene el mapa bajo maxKeys: barre ventanas vencidas a lo más una vez por segundo
// (no en cada intento) y, si sigue lleno, saca llaves cualquiera (salvo strict). Dice si hay lugar.
func (l *limiter) makeRoom(now time.Time) bool {
	if len(l.hits) < maxKeys {
		return true
	}
	if now.Sub(l.swept) > time.Second {
		l.swept = now
		for k, w := range l.hits {
			if now.After(w.reset) {
				delete(l.hits, k)
			}
		}
	}
	if l.strict {
		return len(l.hits) < maxKeys
	}
	for k := range l.hits {
		if len(l.hits) < maxKeys {
			return true
		}
		delete(l.hits, k)
	}
	return true
}
