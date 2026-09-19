package service

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// AmbangEksporAsync jumlah baris di atas ini → job asinkron (05 §3.12).
const AmbangEksporAsync = 50000

// ExportJobStore menyimpan hasil ekspor asinkron di memori proses.
type ExportJobStore struct {
	mu   sync.RWMutex
	jobs map[string]*ExportJob
}

// ExportJob status satu job ekspor.
type ExportJob struct {
	ID        string
	UserID    uint64
	Status    string // pending|ready|failed
	Filename  string
	Body      []byte
	Error     string
	CreatedAt time.Time
}

func NewExportJobStore() *ExportJobStore {
	return &ExportJobStore{jobs: map[string]*ExportJob{}}
}

func (s *ExportJobStore) Mulai(userID uint64, filename string, build func() ([]byte, error)) string {
	id := uuid.NewString()
	job := &ExportJob{
		ID: id, UserID: userID, Status: "pending", Filename: filename, CreatedAt: time.Now().UTC(),
	}
	s.mu.Lock()
	s.jobs[id] = job
	s.mu.Unlock()

	go func() {
		body, err := build()
		s.mu.Lock()
		defer s.mu.Unlock()
		j := s.jobs[id]
		if j == nil {
			return
		}
		if err != nil {
			j.Status = "failed"
			j.Error = err.Error()
			return
		}
		j.Status = "ready"
		j.Body = body
	}()
	return id
}

func (s *ExportJobStore) Ambil(id string, userID uint64) (*ExportJob, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	j, ok := s.jobs[id]
	if !ok || j.UserID != userID {
		return nil, false
	}
	cp := *j
	return &cp, true
}
