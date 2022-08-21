# Frequently Asked Questions (FAQ)

## Design

#### Why does sing-box not have feature X?

Every program contains novel features and omits someone's favorite feature. sing-box is designed with an eye to the needs of performance, lightweight, usability, modularity, and code quality. Your favorite feature may be missing because it doesn't fit, because it compromises performance or design clarity, or because it's a bad idea.

If it bothers you that sing-box is missing feature X, please forgive us and investigate the features that sing-box does have. You might find that they compensate in interesting ways for the lack of X. 

#### Fake IP

Fake IP (also called Fake DNS) is an invasive and imperfect DNS solution that breaks expected behavior, causes DNS leaks and makes many software unusable. It is recommended by some software that lacks DNS processing and caching, but sing-box does not need this.

#### Naive outbound

NaïveProxy's main function is chromium's network stack, and it makes no sense to implement only its transport protocol.

#### Protocol combinations

The "underlying transport" in v2ray-core is actually a combination of a number of proprietary protocols and uses the names of their upstream protocols, resulting in a great deal of Linguistic corruption.

For example, Trojan with v2ray's proprietary gRPC protocol, called Trojan gRPC by the v2ray community, is not actually a protocol and has no role outside of abusing CDNs.