package hysteria

func FragUDPMessage(m UDPMessage, maxSize int) []UDPMessage {
	if m.Size() <= maxSize {
		return []UDPMessage{m}
	}
	fullPayload := m.Data
	maxPayloadSize := maxSize - m.HeaderSize()
	off := 0
	fragID := uint8(0)
	fragCount := uint8((len(fullPayload) + maxPayloadSize - 1) / maxPayloadSize) // round up
	var frags []UDPMessage
	for off < len(fullPayload) {
		payloadSize := len(fullPayload) - off
		if payloadSize > maxPayloadSize {
			payloadSize = maxPayloadSize
		}
		frag := m
		frag.FragID = fragID
		frag.FragCount = fragCount
		frag.Data = fullPayload[off : off+payloadSize]
		frags = append(frags, frag)
		off += payloadSize
		fragID++
	}
	return frags
}

type Defragger struct {
	msgID uint16
	frags []*UDPMessage
	count uint8
}

func (d *Defragger) Feed(m UDPMessage) *UDPMessage {
	if m.FragCount <= 1 {
		return &m
	}
	if m.FragID >= m.FragCount {
		// wtf is this?
		return nil
	}
	if m.MsgID != d.msgID {
		// new message, clear previous state
		d.msgID = m.MsgID
		d.frags = make([]*UDPMessage, m.FragCount)
		d.count = 1
		d.frags[m.FragID] = &m
	} else if d.frags[m.FragID] == nil {
		d.frags[m.FragID] = &m
		d.count++
		if int(d.count) == len(d.frags) {
			// all fragments received, assemble
			var data []byte
			for _, frag := range d.frags {
				data = append(data, frag.Data...)
			}
			m.Data = data
			m.FragID = 0
			m.FragCount = 1
			return &m
		}
	}
	return nil
}
