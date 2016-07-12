package ds

import (
	"fmt"
	"hash"
	"sync"

	"github.com/hashicorp/golang-lru"
	rh "github.com/danmcgoo/scraper/rom/hash"
)

// Hasher is a thread-safe object to hash files. Hashes are cached and
// multiple calls to hash the same file wait for the first call to complete to read from cache.
type Hasher struct {
	h  func() hash.Hash
	c  *lru.Cache
	cl *sync.Mutex
	l  map[string]*sync.Mutex
	b  chan []byte
}

// Hash returns the hash of the file at the given path.
func (h *Hasher) Hash(p string) (string, error) {
	chash, ok := h.c.Get(p)
	if ok {
		switch chash := chash.(type) {
		default:
			return "", fmt.Errorf("unexpected type %T", chash)
		case string:
			return chash, nil
		case error:
			return "", chash
		}
	}
	hl, ok := h.getPathLock(p)
	if ok {
		hl.Lock()
		hl.Unlock()
		return h.Hash(p)
	}
	defer h.deletePathLock(p)
	b := <-h.b
	phash, err := rh.Hash(p, h.h(), b)
	h.b <- b
	if err != nil {
		h.c.Add(p, err)
		return "", err
	}
	h.c.Add(p, phash)
	return phash, nil
}

func (h *Hasher) getPathLock(p string) (*sync.Mutex, bool) {
	h.cl.Lock()
	defer h.cl.Unlock()
	hl, ok := h.l[p]
	if !ok {
		hl = &sync.Mutex{}
		hl.Lock()
		h.l[p] = hl
	}
	return hl, ok
}

func (h *Hasher) deletePathLock(p string) {
	h.cl.Lock()
	defer h.cl.Unlock()
	hl := h.l[p]
	hl.Unlock()
	delete(h.l, p)
}

// NewHasher creates a new Hasher that hashes using the provided hash.
// threads is the expected number of threads a 4MB buffer will be created for each.
func NewHasher(hashFunc func() hash.Hash, threads int) (*Hasher, error) {
	c, err := lru.New(500)
	if err != nil {
		return nil, err
	}
	l := make(map[string]*sync.Mutex)
	b := make(chan []byte, threads)
	for i := 0; i < threads; i++ {
		b <- make([]byte, 1*1024*1024)
	}
	return &Hasher{h: hashFunc, c: c, cl: &sync.Mutex{}, l: l, b: b}, nil
}
