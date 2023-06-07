# FakeIP

FakeIP 是指同时劫持 DNS 和连接请求的程序中的一种行为。它通过虚拟结果响应 DNS 请求，在接受连接时恢复映射。

#### 优点

*

#### 限制

* 它的机制会破坏依赖于返回正确远程地址的应用程序。
* 仅支持 A 和 AAAA（IP）请求，这可能会破坏依赖于其他请求的应用程序。

#### 建议

* 启用 `dns.independent_cache` 除非您始终远程解析 FakeIP 域。
* 如果使用 tun，请确保 tun 路由中包含 FakeIP 地址范围。
* 启用 `experimental.clash_api.store_fakeip` 以持久化 FakeIP 记录，或者使用 `dns.rules.rewrite_ttl` 避免程序重启后在 DNS 被缓存的环境中丢失记录。
