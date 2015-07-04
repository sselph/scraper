package main

import (
	"bufio"
	"crypto/rand"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/sselph/scraper/ds"
	"github.com/syndtr/goleveldb/leveldb"
)

const (
	dbName = "mame_history"
)

func mapper(r rune) rune {
	if r == '\r' {
		return -1
	}
	return r
}

type Entry struct {
	Name   string
	Clones []string
	Bio    string
	System string
}

type Scanner struct {
	r *bufio.Reader
	f io.ReadCloser
}

func New(f io.ReadCloser) *Scanner {
	return &Scanner{bufio.NewReader(f), f}
}

func (s *Scanner) Scan() (Entry, error) {
	e := Entry{}
	for {
		d, err := s.r.ReadBytes('$')
		if err != nil {
			return e, err
		}
		if d[len(d)-2] != '\n' {
			continue
		}
		b, err := s.r.Peek(3)
		if err != nil {
			return e, err
		}
		if string(b) == "end" {
			continue
		}
		break
	}
	l, err := s.r.ReadString('\n')
	if err != nil {
		return e, err
	}
	l = strings.Trim(l, "\n\r")
	p := strings.Split(l, "=")
	if len(p) != 2 {
		return e, fmt.Errorf("invalid info line: %s", l)
	}
	e.System = p[0]
	p = strings.Split(p[1], ",")
	e.Name = p[0]
	e.Clones = append(e.Clones, p[1:]...)
	l, err = s.r.ReadString('\n')
	if err != nil {
		return e, err
	}
	l = strings.Trim(l, "\n\r")
	if l != "$bio" {
		return e, fmt.Errorf("unexpected line: %s", l)
	}
	l, err = s.r.ReadString('\n')
	if err != nil {
		return e, err
	}
	l = strings.Trim(l, "\n\r")
	if l != "" {
		return e, fmt.Errorf("expected blank line after bio: %s", l)
	}
	for {
		l, err = s.r.ReadString('\n')
		if err != nil {
			return e, err
		}
		l = strings.Trim(l, "\n\r")
		if strings.Contains(l, "(c)") {
			break
		}
	}
	for {
		l, err = s.r.ReadString('\n')
		if err != nil {
			return e, err
		}
		l = strings.Trim(l, "\n\r")
		if l == "" {
			break
		}
	}
	for {
		d, err := s.r.ReadString('$')
		if err != nil {
			return e, err
		}
		if len(d) <= 2 || d[len(d)-2] != '\n' {
			continue
		}
		b, err := s.r.Peek(3)
		if err != nil {
			return e, err
		}
		if string(b) == "end" {
			d = d[:len(d)-2]
			e.Bio = e.Bio + d
			e.Bio = strings.Map(mapper, e.Bio)
			return e, nil
		}
		e.Bio = e.Bio + d
	}
}

func main() {
	bioRE := regexp.MustCompile(`- (CAST|CONTRIBUTE|PORTS|SCORING|SERIES|STAFF|TECHNICAL|TRIVIA|UPDATES) -`)
	f, err := os.Open("history.dat")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	scanner := New(f)
	p, err := ds.DefaultCachePath()
	if err != nil {
		log.Fatal(err)
	}
	os.RemoveAll(filepath.Join(p, dbName))
	if err != nil {
		log.Fatal(err)
	}
	ldb, err := leveldb.OpenFile(filepath.Join(p, dbName), nil)
	if err != nil {
		log.Fatal(err)
	}
	defer ldb.Close()
	var count int
	for {
		e, err := scanner.Scan()
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Fatal(err)
		}
		rID := make([]byte, 4)
		for {
			_, err = rand.Read(rID)
			if err != nil {
				log.Fatal("error:", err)
			}
			ok, err := ldb.Has(rID, nil)
			if err != nil {
				log.Fatal(err)
			}
			if !ok {
				break
			}
		}
		batch := new(leveldb.Batch)
		i := []byte(e.Name)
		bparts := bioRE.Split(e.Bio, 2)
		d := []byte(bparts[0])
		batch.Put(i, rID)
		for _, cs := range e.Clones {
			c := []byte(cs)
			batch.Put(c, rID)
		}
		batch.Put(rID, d)
		err = ldb.Write(batch, nil)
		if err != nil {
			log.Fatal(err)
		}
		count++
	}
	fmt.Printf("Found: %d\n", count)
}
