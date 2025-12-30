package sudoku

import "fmt"

type TableSet struct {
	Tables []*Table
}

func NewTableSet(key string, mode string, patterns []string) (*TableSet, error) {
	if len(patterns) == 0 {
		t, err := NewTableWithCustom(key, mode, "")
		if err != nil {
			return nil, err
		}
		return &TableSet{Tables: []*Table{t}}, nil
	}

	tables := make([]*Table, 0, len(patterns))
	for i, pattern := range patterns {
		t, err := NewTableWithCustom(key, mode, pattern)
		if err != nil {
			return nil, fmt.Errorf("build table[%d] (%q): %w", i, pattern, err)
		}
		tables = append(tables, t)
	}
	return &TableSet{Tables: tables}, nil
}

func (ts *TableSet) Candidates() []*Table {
	if ts == nil {
		return nil
	}
	return ts.Tables
}

