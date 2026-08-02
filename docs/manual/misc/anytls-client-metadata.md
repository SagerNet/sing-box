---
icon: material/incognito
---

# AnyTLS client metadata

The AnyTLS protocol has a design flaw: its settings frame requires the client
to send its software name and version to the server, and the protocol
specification requires that clients not disguise this information.

This field serves no protocol purpose — AnyTLS already has a separate version
field for compatibility negotiation, and the open-source server implementation
does not use client metadata. However, the field allows vendors to collect and track client types, and for
platform-specific clients, potentially infer private information such as the operating system type and version range —
something that should not, and is not expected by users to, appear in an
anti-censorship protocol. We have received reports that
commercial proxy providers use this information to identify and block
connections from the official library provided by AnyTLS for sing-box
integration, reportedly because abusive users connect to their servers with
sing-box or with clients using the same official library. This indicates that
client metadata is being collected and used for discrimination in practice.

The protocol specification states that "disguising it has no value." We
disagree: the situation is analogous to browsers implementing TLS ECH GREASE —
without it, privacy-protecting clients can be fingerprinted and treated
differently.

## Status

### 2025-02-20

We merged the
[pull request adding this protocol](https://github.com/SagerNet/sing-box/pull/2615).
Since the metadata was fixed at `sing-anytls/<library version>` in the
implementation provided for our use, and we did not carefully review the
protocol specification and other implementations, we wrongly believed that it
was not private information.

### 2025-04-05

The protocol document
[added](https://github.com/anytls/anytls-go/commit/8812aae7ab29dd88bb89067b9ca676e2e7e29171)
the requirement that third-party implementations fill in the real software
name and version, claiming that "disguising it has no value".

### 2026-07-18

A [pull request submitted to sing-box](https://github.com/SagerNet/sing-box/pull/4311)
was found to additionally upload the `sing-box` name and the actual version;
the change was subsequently reverted and was never released.

### 2026-08-03

sing-box 1.13.16 and 1.14.0-beta.5 have been released; the client metadata in
AnyTLS requests is now empty by default. For compatibility, the
[client_metadata](/configuration/outbound/anytls/#client_metadata) outbound
option allows users to set a custom value.

Since the open-source server implementation does not use this information and
it has no legitimate use, this is not considered a breaking change.

## Recommendations

We recommend that the AnyTLS protocol remove the client metadata, or replace
it with an option that is not sent by default and can be customized by the
user; and that other client implementations also take action, to jointly stop
statistics collection and discrimination based on client metadata.
