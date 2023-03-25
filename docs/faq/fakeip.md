# FakeIP

FakeIP refers to a type of behavior in a program that simultaneously hijacks both DNS and connection requests. It
responds to DNS requests with virtual results and restores mapping when accepting connections.

#### Advantage

* Retrieve the requested domain in places like IP routing (L3) where traffic detection is not possible to assist with routing.
* Decrease an RTT on the first TCP request to a domain (the most common reason).

#### Limitation

* Its mechanism breaks applications that depend on returning correct remote addresses.
* Only A and AAAA (IP) requests are supported, which may break applications that rely on other requests.

#### Recommendation

* Do not use if you do not need L3 routing.
* If using tun, make sure FakeIP ranges is included in the tun's routes.
* Enable `experimental.clash_api.store_fakeip` to persist FakeIP records, or use `dns.rules.rewrite_ttl` to avoid losing records after program restart in DNS cached environments.
