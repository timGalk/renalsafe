// Package audit provides a local append-only journal with corruption detection.
// Hash chains do not authenticate authors or prevent complete privileged rewrites.
package audit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"renalsafe/core/internal/domain"
)

type Event struct {
	Sequence   int               `json:"sequence"`
	Previous   string            `json:"previous_hash"`
	Assessment domain.Assessment `json:"assessment"`
	Knowledge  domain.Knowledge  `json:"knowledge"`
	Hash       string            `json:"hash"`
}

func (e Event) digest() string { e.Hash = ""; return domain.Digest(e) }
func Verify(b []byte) ([]Event, error) {
	events := []Event{}
	prev := ""
	if len(b) > 0 && b[len(b)-1] != '\n' {
		return nil, fmt.Errorf("incomplete audit tail")
	}
	for _, line := range bytes.Split(bytes.TrimSuffix(b, []byte{'\n'}), []byte{'\n'}) {
		if len(b) == 0 {
			break
		}
		var e Event
		if err := domain.Decode(line, &e); err != nil {
			return nil, err
		}
		if e.Sequence != len(events)+1 || e.Previous != prev || e.Hash == "" || e.Hash != e.digest() || e.Assessment.KnowledgeDigest != domain.Digest(e.Knowledge) {
			return nil, fmt.Errorf("audit chain invalid")
		}
		events = append(events, e)
		prev = e.Hash
	}
	return events, nil
}
func Append(path string, a domain.Assessment, k domain.Knowledge) (Event, error) {
	lock, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return Event{}, fmt.Errorf("audit lock: %w", err)
	}
	defer os.Remove(path + ".lock")
	defer lock.Close()
	b, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return Event{}, err
	}
	es, err := Verify(b)
	if err != nil {
		return Event{}, err
	}
	e := Event{Sequence: len(es) + 1, Assessment: a, Knowledge: k}
	if a.KnowledgeDigest != domain.Digest(k) {
		return Event{}, fmt.Errorf("audit knowledge mismatch")
	}
	if len(es) > 0 {
		e.Previous = es[len(es)-1].Hash
	}
	e.Hash = e.digest()
	line, err := json.Marshal(e)
	if err != nil {
		return Event{}, err
	}
	line = append(line, '\n')
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return Event{}, err
	}
	n, writeErr := f.Write(line)
	if writeErr == nil && n != len(line) {
		writeErr = fmt.Errorf("short audit write")
	}
	if writeErr == nil {
		writeErr = f.Sync()
	}
	closeErr := f.Close()
	if writeErr != nil {
		return Event{}, writeErr
	}
	if closeErr != nil {
		return Event{}, closeErr
	}
	return e, nil
}
