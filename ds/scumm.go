package ds

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/sselph/scraper/gdb"
)

// ScummVM is a data source using GDB for ScummVM games.
type ScummVM struct {
	HM     *HashMap
	APIKey string
}

// getID gets the ID from the path..
func (s *ScummVM) getID(p string) (string, error) {
	if filepath.Ext(p) != ".svm" {
		return "", ErrNotFound
	}
	b := filepath.Base(p)
	svm := b[:len(b)-len(filepath.Ext(b))]
	gameID := strings.Split(svm, "-")[0]
	id, ok := s.HM.ID(gameID)
	if !ok {
		return "", ErrNotFound
	}
	return id, nil
}

// GetName implements DS.
func (s *ScummVM) GetName(p string) string {
	b := filepath.Base(p)
	svm := b[:len(b)-len(filepath.Ext(b))]
	gameID := strings.Split(svm, "-")[0]
	n, ok := s.HM.Name(gameID)
	if !ok {
		return ""
	}
	return n
}

// GetGame implements DS.
func (s *ScummVM) GetGame(ctx context.Context, p string) (*Game, error) {
	id, err := s.getID(p)
	if err != nil {
		return nil, err
	}
	req := gdb.GGReq{ID: id}
	resp, err := gdb.GetGame(ctx, s.APIKey, req)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, fmt.Errorf("game with id (%s) not found", id)
	}

	result := ParseGDBGame(*resp)
	return result, nil
}
