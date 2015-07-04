package ds

import (
	"hash"
	"sync"

	"github.com/hashicorp/golang-lru"
	rh "github.com/sselph/scraper/rom/hash"
)

// Hasher is a thread-safe object to hash files. Hashes are cached and
// multiple calls to hash the same file wait for the first call to complete to read from cache.
type Hasher struct {
	h  func() hash.Hash
	c  *lru.Cache
	cl *sync.Mutex
	l  map[string]*sync.Mutex
}

// Hash returns the hash of the file at the given path.
func (h *Hasher) Hash(p string) (string, error) {
	chash, ok := h.c.Get(p)
	if ok {
		return chash.(string), nil
	}
	h.cl.Lock()
	hl, ok := h.l[p]
	if ok {
		h.cl.Unlock()
		hl.Lock()
		hl.Unlock()
		return h.Hash(p)
	}
	hl = &sync.Mutex{}
	hl.Lock()
	h.l[p] = hl
	h.cl.Unlock()
	phash, err := rh.Hash(p, h.h())
	if err != nil {
		hl.Unlock()
		return "", err
	}
	h.c.Add(p, phash)
	hl.Unlock()
	h.cl.Lock()
	delete(h.l, p)
	h.cl.Unlock()
	return phash, nil
}

// NewHasher creates a new Hasher that hashes using the provided hash.
func NewHasher(hashFunc func() hash.Hash) (*Hasher, error) {
	c, err := lru.New(500)
	if err != nil {
		return nil, err
	}
	l := make(map[string]*sync.Mutex)
	return &Hasher{h: hashFunc, c: c, cl: &sync.Mutex{}, l: l}, nil
}
