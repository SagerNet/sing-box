package libbox

import (
	"sync"

	"github.com/sagernet/sing-box/experimental/deprecated"
	"github.com/sagernet/sing/common"
)

var _ deprecated.Manager = (*deprecatedManager)(nil)

type deprecatedManager struct {
	access sync.Mutex
	notes  []deprecated.Note
}

func (m *deprecatedManager) ReportDeprecated(feature deprecated.Note) {
	m.access.Lock()
	defer m.access.Unlock()
	m.notes = common.Uniq(append(m.notes, feature))
}

func (m *deprecatedManager) Get() []deprecated.Note {
	m.access.Lock()
	defer m.access.Unlock()
	notes := m.notes
	m.notes = nil
	return notes
}

var _ = deprecated.Note(DeprecatedNote{})

type DeprecatedNote struct {
	Name              string
	Description       string
	DeprecatedVersion string
	ScheduledVersion  string
	EnvName           string
	MigrationLink     string
}

func (n DeprecatedNote) Impending() bool {
	return deprecated.Note(n).Impending()
}

func (n DeprecatedNote) Message() string {
	return deprecated.Note(n).Message()
}

func (n DeprecatedNote) MessageWithLink() string {
	return deprecated.Note(n).MessageWithLink()
}

type DeprecatedNoteIterator interface {
	HasNext() bool
	Next() *DeprecatedNote
}
