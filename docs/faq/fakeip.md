# FakeIP

FakeIP refers to a type of behavior in a program that simultaneously hijacks both DNS and connection requests. It
responds to DNS requests with virtual results and restores mapping when accepting connections.

#### Advantage

*

#### Limitation

* Its mechanism breaks applications that depend on returning correct remote addresses.
* Only A and AAAA (IP) requests are supported, which may break applications that rely on other requests.

#### Recommendation

* Enable `dns.independent_cache` unless you always resolve FakeIP domains remotely.
* If using tun, make sure FakeIP ranges is included in the tun's routes.
* Enable `experimental.clash_api.store_fakeip` to persist FakeIP records, or use `dns.rules.rewrite_ttl` to avoid losing records after program restart in DNS cached environments.
