package adapter

type NetworkNamespaceManager interface {
	ResolvePath(nameOrPath string) string
}
