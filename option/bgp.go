package option

type BgpOptions struct {
	Address string          `json:"address,omitempty"`
	Port    int32           `json:"port,omitempty"`
	Peer    *BgpPeerOptions `json:"peer,omitempty"`
}

type BgpPeerOptions struct {
	Enable          bool   `json:"enable,omitempty"`
	NeighborAddress string `json:"address,omitempty"`
	LocalAsn        uint32 `json:"localAsn,omitempty"`
	PeerAsn         uint32 `json:"peerAsn,omitempty"`
	AuthPassword    string `json:"password,omitempty"`
}
