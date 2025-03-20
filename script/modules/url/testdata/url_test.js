"use strict";

const assert = require("assert.js");

function testURLCtor(str, expected) {
  assert.sameValue(new URL(str).toString(), expected);
}

function testURLCtorBase(ref, base, expected, message) {
  assert.sameValue(new URL(ref, base).toString(), expected, message);
}

testURLCtorBase("https://example.org/", undefined, "https://example.org/");
testURLCtorBase("/foo", "https://example.org/", "https://example.org/foo");
testURLCtorBase("http://Example.com/", "https://example.org/", "http://example.com/");
testURLCtorBase("https://Example.com/", "https://example.org/", "https://example.com/");
testURLCtorBase("foo://Example.com/", "https://example.org/", "foo://Example.com/");
testURLCtorBase("foo:Example.com/", "https://example.org/", "foo:Example.com/");
testURLCtorBase("#hash", "https://example.org/", "https://example.org/#hash");

testURLCtor("HTTP://test.com", "http://test.com/");
testURLCtor("HTTPS://á.com", "https://xn--1ca.com/");
testURLCtor("HTTPS://á.com:123", "https://xn--1ca.com:123/");
testURLCtor("https://test.com#asdfá", "https://test.com/#asdf%C3%A1");
testURLCtor("HTTPS://á.com:123/á", "https://xn--1ca.com:123/%C3%A1");
testURLCtor("fish://á.com", "fish://%C3%A1.com");
testURLCtor("https://test.com/?a=1 /2", "https://test.com/?a=1%20/2");
testURLCtor("https://test.com/á=1?á=1&ü=2#é", "https://test.com/%C3%A1=1?%C3%A1=1&%C3%BC=2#%C3%A9");

assert.throws(() => new URL("test"), TypeError);
assert.throws(() => new URL("ssh://EEE:ddd"), TypeError);

{
    let u = new URL("https://example.org/");
    assert.sameValue(u.__proto__.constructor, URL);
    assert.sameValue(u instanceof URL, true);
}

{
    let u = new URL("https://example.org/");
    assert.sameValue(u.searchParams, u.searchParams);
}

let myURL;

// Hash
myURL = new URL("https://example.org/foo#bar");
myURL.hash = "baz";
assert.sameValue(myURL.href, "https://example.org/foo#baz");

myURL.hash = "#baz";
assert.sameValue(myURL.href, "https://example.org/foo#baz");

myURL.hash = "#á=1 2";
assert.sameValue(myURL.href, "https://example.org/foo#%C3%A1=1%202");

myURL.hash = "#a/#b";
// FAILING: the second # gets escaped
//assert.sameValue(myURL.href, "https://example.org/foo#a/#b");
assert.sameValue(myURL.search, "");
// FAILING: the second # gets escaped
//assert.sameValue(myURL.hash, "#a/#b");

// Host
myURL = new URL("https://example.org:81/foo");
myURL.host = "example.com:82";
assert.sameValue(myURL.href, "https://example.com:82/foo");

// Hostname
myURL = new URL("https://example.org:81/foo");
myURL.hostname = "example.com:82";
assert.sameValue(myURL.href, "https://example.org:81/foo");

myURL.hostname = "á.com";
assert.sameValue(myURL.href, "https://xn--1ca.com:81/foo");

// href
myURL = new URL("https://example.org/foo");
myURL.href = "https://example.com/bar";
assert.sameValue(myURL.href, "https://example.com/bar");

// Password
myURL = new URL("https://abc:xyz@example.com");
myURL.password = "123";
assert.sameValue(myURL.href, "https://abc:123@example.com/");

// pathname
myURL = new URL("https://example.org/abc/xyz?123");
myURL.pathname = "/abcdef";
assert.sameValue(myURL.href, "https://example.org/abcdef?123");

myURL.pathname = "";
assert.sameValue(myURL.href, "https://example.org/?123");

myURL.pathname = "á";
assert.sameValue(myURL.pathname, "/%C3%A1");
assert.sameValue(myURL.href, "https://example.org/%C3%A1?123");

// port

myURL = new URL("https://example.org:8888");
assert.sameValue(myURL.port, "8888");

function testSetPort(port, expected) {
  const url = new URL("https://example.org:8888");
  url.port = port;
  assert.sameValue(url.port, expected);
}

testSetPort(0, "0");
testSetPort(-0, "0");

// Default ports are automatically transformed to the empty string
// (HTTPS protocol's default port is 443)
testSetPort("443", "");
testSetPort(443, "");

// Empty string is the same as default port
testSetPort("", "");

// Completely invalid port strings are ignored
testSetPort("abcd", "8888");
testSetPort("-123", "");
testSetPort(-123, "");
testSetPort(-123.45, "");
testSetPort(undefined, "8888");
testSetPort(null, "8888");
testSetPort(+Infinity, "8888");
testSetPort(-Infinity, "8888");
testSetPort(NaN, "8888");

// Leading numbers are treated as a port number
testSetPort("5678abcd", "5678");
testSetPort("a5678abcd", "");

// Non-integers are truncated
testSetPort(1234.5678, "1234");

// Out-of-range numbers which are not represented in scientific notation
// will be ignored.
testSetPort(1e10, "8888");
testSetPort("123456", "8888");
testSetPort(123456, "8888");
testSetPort(4.567e21, "4");

// toString() takes precedence over valueOf(), even if it returns a valid integer
testSetPort(
  {
    toString() {
      return "2";
    },
    valueOf() {
      return 1;
    },
  },
  "2"
);

// Protocol
function testSetProtocol(url, protocol, expected) {
  url.protocol = protocol;
  assert.sameValue(url.protocol, expected);
}
testSetProtocol(new URL("https://example.org"), "ftp", "ftp:");
testSetProtocol(new URL("https://example.org"), "ftp:", "ftp:");
testSetProtocol(new URL("https://example.org"), "FTP:", "ftp:");
testSetProtocol(new URL("https://example.org"), "ftp: blah", "ftp:");
// special to non-special
testSetProtocol(new URL("https://example.org"), "foo", "https:");
// non-special to special
testSetProtocol(new URL("fish://example.org"), "https", "fish:");

// Search
myURL = new URL("https://example.org/abc?123");
myURL.search = "abc=xyz";
assert.sameValue(myURL.href, "https://example.org/abc?abc=xyz");

myURL.search = "a=1 2";
assert.sameValue(myURL.href, "https://example.org/abc?a=1%202");

myURL.search = "á=ú";
assert.sameValue(myURL.search, "?%C3%A1=%C3%BA");
assert.sameValue(myURL.href, "https://example.org/abc?%C3%A1=%C3%BA");

myURL.hash = "hash";
myURL.search = "a=#b";
assert.sameValue(myURL.href, "https://example.org/abc?a=%23b#hash");
assert.sameValue(myURL.search, "?a=%23b");
assert.sameValue(myURL.hash, "#hash");

// Username
myURL = new URL("https://abc:xyz@example.com/");
myURL.username = "123";
assert.sameValue(myURL.href, "https://123:xyz@example.com/");

// Origin, read-only
assert.throws(() => {
  myURL.origin = "abc";
}, TypeError);

// href
myURL = new URL("https://example.org");
myURL.href = "https://example.com";
assert.sameValue(myURL.href, "https://example.com/");

assert.throws(() => {
  myURL.href = "test";
}, TypeError);

// Search Params
myURL = new URL("https://example.com/");
myURL.searchParams.append("user", "abc");
assert.sameValue(myURL.toString(), "https://example.com/?user=abc");
myURL.searchParams.append("first", "one");
assert.sameValue(myURL.toString(), "https://example.com/?user=abc&first=one");
myURL.searchParams.delete("user");
assert.sameValue(myURL.toString(), "https://example.com/?first=one");

{
    const url = require("url");

    assert.sameValue(url.domainToASCII('español.com'), "xn--espaol-zwa.com");
    assert.sameValue(url.domainToASCII('中文.com'), "xn--fiq228c.com");
    assert.sameValue(url.domainToASCII('xn--iñvalid.com'), "");

    assert.sameValue(url.domainToUnicode('xn--espaol-zwa.com'), "español.com");
    assert.sameValue(url.domainToUnicode('xn--fiq228c.com'), "中文.com");
    assert.sameValue(url.domainToUnicode('xn--iñvalid.com'), "");
}
