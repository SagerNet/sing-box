package dns

import (
	"context"
	"net"
	"net/netip"
	"strings"
	"time"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/cache"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/task"

	"golang.org/x/net/dns/dnsmessage"
)

const DefaultTTL = 600

var (
	ErrNoRawSupport = E.New("no raw query support by current transport")
	ErrNotCached    = E.New("not cached")
)

var _ adapter.DNSClient = (*Client)(nil)

type Client struct {
	cache         *cache.LruCache[dnsmessage.Question, *dnsmessage.Message]
	disableCache  bool
	disableExpire bool
}

func NewClient(options option.DNSClientOptions) *Client {
	if options.DisableCache {
		return &Client{
			disableCache: true,
		}
	} else {
		return &Client{
			cache:         cache.New[dnsmessage.Question, *dnsmessage.Message](),
			disableExpire: options.DisableExpire,
		}
	}
}

func (c *Client) Exchange(ctx context.Context, transport adapter.DNSTransport, message *dnsmessage.Message) (*dnsmessage.Message, error) {
	if len(message.Questions) == 0 {
		return nil, E.New("empty query")
	}
	question := message.Questions[0]
	if !c.disableCache {
		cachedAnswer, cached := c.cache.Load(question)
		if cached {
			cachedAnswer.ID = message.ID
			return cachedAnswer, nil
		}
	}
	if !transport.Raw() {
		if question.Type == dnsmessage.TypeA || question.Type == dnsmessage.TypeAAAA {
			return c.exchangeToLookup(ctx, transport, message, question)
		}
		return nil, ErrNoRawSupport
	}
	messageId := message.ID
	response, err := transport.Exchange(ctx, message)
	if err != nil {
		return nil, err
	}
	response.ID = messageId
	if !c.disableCache {
		c.storeCache(question, response)
	}
	return response, err
}

func (c *Client) Lookup(ctx context.Context, transport adapter.DNSTransport, domain string, strategy C.DomainStrategy) ([]netip.Addr, error) {
	if strings.HasPrefix(domain, ".") {
		domain = domain[:len(domain)-1]
	}
	dnsName, err := dnsmessage.NewName(domain + ".")
	if err != nil {
		return nil, wrapError(err)
	}
	if transport.Raw() {
		if strategy == C.DomainStrategyUseIPv4 {
			return c.lookupToExchange(ctx, transport, dnsName, dnsmessage.TypeA)
		} else if strategy == C.DomainStrategyUseIPv6 {
			return c.lookupToExchange(ctx, transport, dnsName, dnsmessage.TypeAAAA)
		}
		var response4 []netip.Addr
		var response6 []netip.Addr
		err = task.Run(ctx, func() error {
			response, err := c.lookupToExchange(ctx, transport, dnsName, dnsmessage.TypeA)
			if err != nil {
				return err
			}
			response4 = response
			return nil
		}, func() error {
			response, err := c.lookupToExchange(ctx, transport, dnsName, dnsmessage.TypeAAAA)
			if err != nil {
				return err
			}
			response6 = response
			return nil
		})
		if len(response4) == 0 && len(response6) == 0 {
			return nil, err
		}
		return sortAddresses(response4, response6, strategy), nil
	}
	if !c.disableCache {
		if strategy == C.DomainStrategyUseIPv4 {
			response, err := c.questionCache(dnsmessage.Question{
				Name:  dnsName,
				Type:  dnsmessage.TypeA,
				Class: dnsmessage.ClassINET,
			})
			if err != ErrNotCached {
				return response, err
			}
		} else if strategy == C.DomainStrategyUseIPv6 {
			response, err := c.questionCache(dnsmessage.Question{
				Name:  dnsName,
				Type:  dnsmessage.TypeAAAA,
				Class: dnsmessage.ClassINET,
			})
			if err != ErrNotCached {
				return response, err
			}
		} else {
			response4, _ := c.questionCache(dnsmessage.Question{
				Name:  dnsName,
				Type:  dnsmessage.TypeA,
				Class: dnsmessage.ClassINET,
			})
			response6, _ := c.questionCache(dnsmessage.Question{
				Name:  dnsName,
				Type:  dnsmessage.TypeAAAA,
				Class: dnsmessage.ClassINET,
			})
			if len(response4) > 0 || len(response6) > 0 {
				return sortAddresses(response4, response6, strategy), nil
			}
		}
	}
	var rCode dnsmessage.RCode
	response, err := transport.Lookup(ctx, domain, strategy)
	if err != nil {
		err = wrapError(err)
		if rCodeError, isRCodeError := err.(RCodeError); !isRCodeError {
			return nil, err
		} else {
			rCode = dnsmessage.RCode(rCodeError)
		}
		if c.disableCache {
			return nil, err
		}
	}
	header := dnsmessage.Header{
		Response:      true,
		Authoritative: true,
		RCode:         rCode,
	}
	if !c.disableCache {
		if strategy != C.DomainStrategyUseIPv6 {
			question4 := dnsmessage.Question{
				Name:  dnsName,
				Type:  dnsmessage.TypeA,
				Class: dnsmessage.ClassINET,
			}
			response4 := common.Filter(response, func(addr netip.Addr) bool {
				return addr.Is4() || addr.Is4In6()
			})
			message4 := &dnsmessage.Message{
				Header:    header,
				Questions: []dnsmessage.Question{question4},
			}
			if len(response4) > 0 {
				for _, address := range response4 {
					message4.Answers = append(message4.Answers, dnsmessage.Resource{
						Header: dnsmessage.ResourceHeader{
							Name:  question4.Name,
							Class: question4.Class,
							TTL:   DefaultTTL,
						},
						Body: &dnsmessage.AResource{
							A: address.As4(),
						},
					})
				}
			}
			c.storeCache(question4, message4)
		}
		if strategy != C.DomainStrategyUseIPv4 {
			question6 := dnsmessage.Question{
				Name:  dnsName,
				Type:  dnsmessage.TypeAAAA,
				Class: dnsmessage.ClassINET,
			}
			response6 := common.Filter(response, func(addr netip.Addr) bool {
				return addr.Is6() && !addr.Is4In6()
			})
			message6 := &dnsmessage.Message{
				Header:    header,
				Questions: []dnsmessage.Question{question6},
			}
			if len(response6) > 0 {
				for _, address := range response6 {
					message6.Answers = append(message6.Answers, dnsmessage.Resource{
						Header: dnsmessage.ResourceHeader{
							Name:  question6.Name,
							Class: question6.Class,
							TTL:   DefaultTTL,
						},
						Body: &dnsmessage.AAAAResource{
							AAAA: address.As16(),
						},
					})
				}
			}
			c.storeCache(question6, message6)
		}
	}
	return response, err
}

func sortAddresses(response4 []netip.Addr, response6 []netip.Addr, strategy C.DomainStrategy) []netip.Addr {
	if strategy == C.DomainStrategyPreferIPv6 {
		return append(response6, response4...)
	} else {
		return append(response4, response6...)
	}
}

func (c *Client) storeCache(question dnsmessage.Question, message *dnsmessage.Message) {
	if c.disableExpire {
		c.cache.Store(question, message)
		return
	}
	timeToLive := DefaultTTL
	for _, answer := range message.Answers {
		if int(answer.Header.TTL) < timeToLive {
			timeToLive = int(answer.Header.TTL)
		}
	}
	expire := time.Now().Add(time.Second * time.Duration(timeToLive))
	c.cache.StoreWithExpire(question, message, expire)
}

func (c *Client) exchangeToLookup(ctx context.Context, transport adapter.DNSTransport, message *dnsmessage.Message, question dnsmessage.Question) (*dnsmessage.Message, error) {
	domain := question.Name.String()
	var strategy C.DomainStrategy
	if question.Type == dnsmessage.TypeA {
		strategy = C.DomainStrategyUseIPv4
	} else {
		strategy = C.DomainStrategyUseIPv6
	}
	var rCode dnsmessage.RCode
	result, err := c.Lookup(ctx, transport, domain, strategy)
	if err != nil {
		err = wrapError(err)
		if rCodeError, isRCodeError := err.(RCodeError); !isRCodeError {
			return nil, err
		} else {
			rCode = dnsmessage.RCode(rCodeError)
		}
	}
	response := dnsmessage.Message{
		Header: dnsmessage.Header{
			ID:                 message.ID,
			RCode:              rCode,
			RecursionAvailable: true,
			RecursionDesired:   true,
			Response:           true,
		},
		Questions: message.Questions,
	}
	for _, address := range result {
		var resource dnsmessage.Resource
		resource.Header = dnsmessage.ResourceHeader{
			Name:  question.Name,
			Class: question.Class,
			TTL:   DefaultTTL,
		}
		if address.Is4() || address.Is4In6() {
			resource.Body = &dnsmessage.AResource{
				A: address.As4(),
			}
		} else {
			resource.Body = &dnsmessage.AAAAResource{
				AAAA: address.As16(),
			}
		}
	}
	return &response, nil
}

func (c *Client) lookupToExchange(ctx context.Context, transport adapter.DNSTransport, name dnsmessage.Name, qType dnsmessage.Type) ([]netip.Addr, error) {
	question := dnsmessage.Question{
		Name:  name,
		Type:  qType,
		Class: dnsmessage.ClassINET,
	}
	if !c.disableCache {
		cachedAddresses, err := c.questionCache(question)
		if err != ErrNotCached {
			return cachedAddresses, err
		}
	}
	message := dnsmessage.Message{
		Header: dnsmessage.Header{
			ID:               0,
			RecursionDesired: true,
		},
		Questions: []dnsmessage.Question{question},
	}
	response, err := c.Exchange(ctx, transport, &message)
	if err != nil {
		return nil, err
	}
	return messageToAddresses(response)
}

func (c *Client) questionCache(question dnsmessage.Question) ([]netip.Addr, error) {
	response, cached := c.cache.Load(question)
	if !cached {
		return nil, ErrNotCached
	}
	return messageToAddresses(response)
}

func messageToAddresses(response *dnsmessage.Message) ([]netip.Addr, error) {
	if response.RCode != dnsmessage.RCodeSuccess {
		return nil, RCodeError(response.RCode)
	}
	addresses := make([]netip.Addr, 0, len(response.Answers))
	for _, answer := range response.Answers {
		switch resource := answer.Body.(type) {
		case *dnsmessage.AResource:
			addresses = append(addresses, netip.AddrFrom4(resource.A))
		case *dnsmessage.AAAAResource:
			addresses = append(addresses, netip.AddrFrom16(resource.AAAA))
		}
	}
	return addresses, nil
}

func wrapError(err error) error {
	if dnsErr, isDNSError := err.(*net.DNSError); isDNSError {
		if dnsErr.IsNotFound {
			return RCodeNameError
		}
	}
	return err
}
