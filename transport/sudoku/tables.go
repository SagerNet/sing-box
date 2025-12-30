package sudoku

import (
	"strings"
	"sync"

	"github.com/sagernet/sing-box/transport/sudoku/crypto"
	"github.com/sagernet/sing-box/transport/sudoku/obfs/sudoku"
)

type tableCacheKey struct {
	key     string
	mode    string
	pattern string
}

var tableCache sync.Map

func cachedTable(key string, mode string, customPattern string) (*sudoku.Table, error) {
	cacheKey := tableCacheKey{
		key:     key,
		mode:    strings.ToLower(strings.TrimSpace(mode)),
		pattern: strings.ToLower(strings.TrimSpace(customPattern)),
	}
	if v, ok := tableCache.Load(cacheKey); ok {
		return v.(*sudoku.Table), nil
	}
	t, err := sudoku.NewTableWithCustom(key, mode, customPattern)
	if err != nil {
		return nil, err
	}
	actual, _ := tableCache.LoadOrStore(cacheKey, t)
	return actual.(*sudoku.Table), nil
}

// NewTablesWithCustomPatterns builds one or more obfuscation tables from x/v/p custom patterns.
// When customTables is non-empty it overrides customTable (matching upstream Sudoku behavior).
func NewTablesWithCustomPatterns(key string, tableType string, customTable string, customTables []string) ([]*sudoku.Table, error) {
	patterns := customTables
	if len(patterns) == 0 && strings.TrimSpace(customTable) != "" {
		patterns = []string{customTable}
	}
	if len(patterns) == 0 {
		patterns = []string{""}
	}

	tables := make([]*sudoku.Table, 0, len(patterns))
	for _, pattern := range patterns {
		t, err := cachedTable(key, tableType, pattern)
		if err != nil {
			return nil, err
		}
		tables = append(tables, t)
	}
	return tables, nil
}

// ClientAEADSeed derives the shared public seed from a client key string.
// If key is an ED25519 split/master private key, it recovers the public key and uses that instead,
// ensuring client(private) and server(public) are wire compatible.
func ClientAEADSeed(key string) string {
	if recovered, err := crypto.RecoverPublicKey(key); err == nil {
		return crypto.EncodePoint(recovered)
	}
	return key
}

func GenKeyPair() (privateKey, publicKey string, err error) {
	pair, err := crypto.GenerateMasterKey()
	if err != nil {
		return "", "", err
	}
	privateKey, err = crypto.SplitPrivateKey(pair.Private)
	if err != nil {
		return "", "", err
	}
	publicKey = crypto.EncodePoint(pair.Public)
	return privateKey, publicKey, nil
}

