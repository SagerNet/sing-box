package libbox

import "github.com/sagernet/sing-box/experimental/deprecated"

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

func (n DeprecatedNote) String() string {
	return deprecated.Note(n).String()
}
