package padding

import (
	"crypto/md5"
	"crypto/rand"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/sagernet/sing-box/transport/anytls/util"
	"github.com/sagernet/sing/common/atomic"
)

const CheckMark = -1

var DefaultPaddingScheme = []byte(`stop=8
0=34-120
1=100-400
2=400-500,c,500-1000,c,400-500,c,500-1000,c,500-1000,c,400-500
3=500-1000
4=500-1000
5=500-1000
6=500-1000
7=500-1000`)

type PaddingFactory struct {
	scheme    util.StringMap
	RawScheme []byte
	Stop      uint32
	Md5       string
}

func UpdatePaddingScheme(rawScheme []byte, to *atomic.TypedValue[*PaddingFactory]) bool {
	if p := NewPaddingFactory(rawScheme); p != nil {
		to.Store(p)
		return true
	}
	return false
}

func NewPaddingFactory(rawScheme []byte) *PaddingFactory {
	p := &PaddingFactory{
		RawScheme: rawScheme,
		Md5:       fmt.Sprintf("%x", md5.Sum(rawScheme)),
	}
	scheme := util.StringMapFromBytes(rawScheme)
	if len(scheme) == 0 {
		return nil
	}
	if stop, err := strconv.Atoi(scheme["stop"]); err == nil {
		p.Stop = uint32(stop)
	} else {
		return nil
	}
	p.scheme = scheme
	return p
}

func (p *PaddingFactory) GenerateRecordPayloadSizes(pkt uint32) (pktSizes []int) {
	if s, ok := p.scheme[strconv.Itoa(int(pkt))]; ok {
		sRanges := strings.Split(s, ",")
		for _, sRange := range sRanges {
			sRangeMinMax := strings.Split(sRange, "-")
			if len(sRangeMinMax) == 2 {
				_min, err := strconv.ParseInt(sRangeMinMax[0], 10, 64)
				if err != nil {
					continue
				}
				_max, err := strconv.ParseInt(sRangeMinMax[1], 10, 64)
				if err != nil {
					continue
				}
				if _min > _max {
					_min, _max = _max, _min
				}
				if _min <= 0 || _max <= 0 {
					continue
				}
				if _min == _max {
					pktSizes = append(pktSizes, int(_min))
				} else {
					i, _ := rand.Int(rand.Reader, big.NewInt(_max-_min))
					pktSizes = append(pktSizes, int(i.Int64()+_min))
				}
			} else if sRange == "c" {
				pktSizes = append(pktSizes, CheckMark)
			}
		}
	}
	return
}
