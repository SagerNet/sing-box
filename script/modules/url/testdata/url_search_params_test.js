"use strict";

const assert = require("assert.js");

let params;

function testCtor(value, expected) {
  assert.sameValue(new URLSearchParams(value).toString(), expected);
}

testCtor("user=abc&query=xyz", "user=abc&query=xyz");
testCtor("?user=abc&query=xyz", "user=abc&query=xyz");

testCtor(
  {
    num: 1,
    user: "abc",
    query: ["first", "second"],
    obj: { prop: "value" },
    b: true,
  },
  "num=1&user=abc&query=first%2Csecond&obj=%5Bobject+Object%5D&b=true"
);

const map = new Map();
map.set("user", "abc");
map.set("query", "xyz");
testCtor(map, "user=abc&query=xyz");

testCtor(
  [
    ["user", "abc"],
    ["query", "first"],
    ["query", "second"],
  ],
  "user=abc&query=first&query=second"
);

// Each key-value pair must have exactly two elements
assert.throwsNodeError(() => new URLSearchParams([["single_value"]]), TypeError, "ERR_INVALID_TUPLE");
assert.throwsNodeError(() => new URLSearchParams([["too", "many", "values"]]), TypeError, "ERR_INVALID_TUPLE");

params = new URLSearchParams("a=b&cc=d");
params.forEach((value, name, searchParams) => {
  if (name === "a") {
    assert.sameValue(value, "b");
  }
  if (name === "cc") {
    assert.sameValue(value, "d");
  }
  assert.sameValue(searchParams, params);
});

params.forEach((value, name, searchParams) => {
    if (name === "a") {
        assert.sameValue(value, "b");
        searchParams.set("cc", "d1");
    }
    if (name === "cc") {
        assert.sameValue(value, "d1");
    }
    assert.sameValue(searchParams, params);
});

assert.throwsNodeError(() => params.forEach(123), TypeError, "ERR_INVALID_ARG_TYPE");

assert.throwsNodeError(() => params.forEach.call(1, 2), TypeError, "ERR_INVALID_THIS");

params = new URLSearchParams("a=1=2&b=3");
assert.sameValue(params.size, 2);
assert.sameValue(params.get("a"), "1=2");
assert.sameValue(params.get("b"), "3");

params = new URLSearchParams("&");
assert.sameValue(params.size, 0);

params = new URLSearchParams("& ");
assert.sameValue(params.size, 1);
assert.sameValue(params.get(" "), "");

params = new URLSearchParams(" &");
assert.sameValue(params.size, 1);
assert.sameValue(params.get(" "), "");

params = new URLSearchParams("=");
assert.sameValue(params.size, 1);
assert.sameValue(params.get(""), "");

params = new URLSearchParams("&=2");
assert.sameValue(params.size, 1);
assert.sameValue(params.get(""), "2");

params = new URLSearchParams("?user=abc");
assert.throwsNodeError(() => params.append(), TypeError, "ERR_MISSING_ARGS");
params.append("query", "first");
assert.sameValue(params.toString(), "user=abc&query=first");

params = new URLSearchParams("first=one&second=two&third=three");
assert.throwsNodeError(() => params.delete(), TypeError, "ERR_MISSING_ARGS");
params.delete("second", "fake-value");
assert.sameValue(params.toString(), "first=one&second=two&third=three");
params.delete("third", "three");
assert.sameValue(params.toString(), "first=one&second=two");
params.delete("second");
assert.sameValue(params.toString(), "first=one");

params = new URLSearchParams("user=abc&query=xyz");
assert.throwsNodeError(() => params.get(), TypeError, "ERR_MISSING_ARGS");
assert.sameValue(params.get("user"), "abc");
assert.sameValue(params.get("non-existant"), null);

params = new URLSearchParams("query=first&query=second");
assert.throwsNodeError(() => params.getAll(), TypeError, "ERR_MISSING_ARGS");
const all = params.getAll("query");
assert.sameValue(all.includes("first"), true);
assert.sameValue(all.includes("second"), true);
assert.sameValue(all.length, 2);
const getAllUndefined = params.getAll(undefined);
assert.sameValue(getAllUndefined.length, 0);
const getAllNonExistant = params.getAll("does_not_exists");
assert.sameValue(getAllNonExistant.length, 0);

params = new URLSearchParams("user=abc&query=xyz");
assert.throwsNodeError(() => params.has(), TypeError, "ERR_MISSING_ARGS");
assert.sameValue(params.has(undefined), false);
assert.sameValue(params.has("user"), true);
assert.sameValue(params.has("user", "abc"), true);
assert.sameValue(params.has("user", "abc", "extra-param"), true);
assert.sameValue(params.has("user", "efg"), false);
assert.sameValue(params.has("user", undefined), true);

params = new URLSearchParams();
params.append("foo", "bar");
params.append("foo", "baz");
params.append("abc", "def");
assert.sameValue(params.toString(), "foo=bar&foo=baz&abc=def");
params.set("foo", "def");
params.set("xyz", "opq");
assert.sameValue(params.toString(), "foo=def&abc=def&xyz=opq");

params = new URLSearchParams("query=first&query=second&user=abc&double=first,second");
const URLSearchIteratorPrototype = params.entries().__proto__;
assert.sameValue(typeof URLSearchIteratorPrototype, "object");

assert.sameValue(params[Symbol.iterator], params.entries);

{
    const entries = params.entries();
    assert.sameValue(entries.toString(), "[object URLSearchParams Iterator]");
    assert.sameValue(entries.__proto__, URLSearchIteratorPrototype);

    let item = entries.next();
    assert.sameValue(item.value.toString(), ["query", "first"].toString());
    assert.sameValue(item.done, false);

    item = entries.next();
    assert.sameValue(item.value.toString(), ["query", "second"].toString());
    assert.sameValue(item.done, false);

    item = entries.next();
    assert.sameValue(item.value.toString(), ["user", "abc"].toString());
    assert.sameValue(item.done, false);

    item = entries.next();
    assert.sameValue(item.value.toString(), ["double", "first,second"].toString());
    assert.sameValue(item.done, false);

    item = entries.next();
    assert.sameValue(item.value, undefined);
    assert.sameValue(item.done, true);
}

params = new URLSearchParams("query=first&query=second&user=abc");
{
    const keys = params.keys();
    assert.sameValue(keys.__proto__, URLSearchIteratorPrototype);

    let item = keys.next();
    assert.sameValue(item.value, "query");
    assert.sameValue(item.done, false);

    item = keys.next();
    assert.sameValue(item.value, "query");
    assert.sameValue(item.done, false);

    item = keys.next();
    assert.sameValue(item.value, "user");
    assert.sameValue(item.done, false);

    item = keys.next();
    assert.sameValue(item.value, undefined);
    assert.sameValue(item.done, true);
}

params = new URLSearchParams("query=first&query=second&user=abc");
{
    const values = params.values();
    assert.sameValue(values.__proto__, URLSearchIteratorPrototype);

    let item = values.next();
    assert.sameValue(item.value, "first");
    assert.sameValue(item.done, false);

    item = values.next();
    assert.sameValue(item.value, "second");
    assert.sameValue(item.done, false);

    item = values.next();
    assert.sameValue(item.value, "abc");
    assert.sameValue(item.done, false);

    item = values.next();
    assert.sameValue(item.value, undefined);
    assert.sameValue(item.done, true);
}


params = new URLSearchParams("query[]=abc&type=search&query[]=123");
params.sort();
assert.sameValue(params.toString(), "query%5B%5D=abc&query%5B%5D=123&type=search");

params = new URLSearchParams("query=first&query=second&user=abc");
assert.sameValue(params.size, 3);

params = new URLSearchParams("%");
assert.sameValue(params.has("%"), true);
assert.sameValue(params.toString(), "%25=");

{
    const params = new URLSearchParams("");
    assert.sameValue(params.size, 0);
    assert.sameValue(params.toString(), "");
    assert.sameValue(params.get(undefined), null);
    params.set(undefined, true);
    assert.sameValue(params.has(undefined), true);
    assert.sameValue(params.has("undefined"), true);
    assert.sameValue(params.get("undefined"), "true");
    assert.sameValue(params.get(undefined), "true");
    assert.sameValue(params.getAll(undefined).toString(), ["true"].toString());
    params.delete(undefined);
    assert.sameValue(params.has(undefined), false);
    assert.sameValue(params.has("undefined"), false);

    assert.sameValue(params.has(null), false);
    params.set(null, "nullval");
    assert.sameValue(params.has(null), true);
    assert.sameValue(params.has("null"), true);
    assert.sameValue(params.get(null), "nullval");
    assert.sameValue(params.get("null"), "nullval");
    params.delete(null);
    assert.sameValue(params.has(null), false);
    assert.sameValue(params.has("null"), false);
}

function* functionGeneratorExample() {
  yield ["user", "abc"];
  yield ["query", "first"];
  yield ["query", "second"];
}

params = new URLSearchParams(functionGeneratorExample());
assert.sameValue(params.toString(), "user=abc&query=first&query=second");

assert.sameValue(params.__proto__.constructor, URLSearchParams);
assert.sameValue(params instanceof URLSearchParams, true);

{
    const params = new URLSearchParams("1=2&1=3");
    assert.sameValue(params.get(1), "2");
    assert.sameValue(params.getAll(1).toString(), ["2", "3"].toString());
    assert.sameValue(params.getAll("x").toString(), [].toString());
}

// Sync
{
    const url = new URL("https://test.com/");
    const params = url.searchParams;
    assert.sameValue(params.size, 0);
    url.search = "a=1";
    assert.sameValue(params.size, 1);
    assert.sameValue(params.get("a"), "1");
}

{
    const url = new URL("https://test.com/?a=1");
    const params = url.searchParams;
    assert.sameValue(params.size, 1);
    url.search = "";
    assert.sameValue(params.size, 0);
    url.search = "b=2";
    assert.sameValue(params.size, 1);
}

{
    const url = new URL("https://test.com/");
    const params = url.searchParams;
    params.append("a", "1");
    assert.sameValue(url.toString(), "https://test.com/?a=1");
}

{
    const url = new URL("https://test.com/");
    url.searchParams.append("a", "1");
    url.searchParams.append("b", "1");
    assert.sameValue(url.toString(), "https://test.com/?a=1&b=1");
}

{
    const url = new URL("https://test.com/");
    const params = url.searchParams;
    url.searchParams.append("a", "1");
    assert.sameValue(url.search, "?a=1");
}

{
    const url = new URL("https://test.com/?a=1");
    const params = url.searchParams;
    params.append("a", "2");
    assert.sameValue(url.search, "?a=1&a=2");
}

{
    const url = new URL("https://test.com/");
    const params = url.searchParams;
    params.set("a", "1");
    assert.sameValue(url.search, "?a=1");
}

{
    const url = new URL("https://test.com/");
    url.searchParams.set("a", "1");
    url.searchParams.set("b", "1");
    assert.sameValue(url.toString(), "https://test.com/?a=1&b=1");
}

{
    const url = new URL("https://test.com/?a=1&b=2");
    const params = url.searchParams;
    params.delete("a");
    assert.sameValue(url.search, "?b=2");
}

{
    const url = new URL("https://test.com/?b=2&a=1");
    const params = url.searchParams;
    params.sort();
    assert.sameValue(url.search, "?a=1&b=2");
}

{
    const url = new URL("https://test.com/?a=1");
    const params = url.searchParams;
    params.delete("a");
    assert.sameValue(url.search, "");

    params.set("a", 2);
    assert.sameValue(url.search, "?a=2");
}

// FAILING: no custom properties on wrapped Go structs
/*
{
    const params = new URLSearchParams("");
    assert.sameValue(Object.isExtensible(params), true);
    assert.sameValue(Reflect.defineProperty(params, "customField", {value: 42, configurable: true}), true);
    assert.sameValue(params.customField, 42);
    const desc = Reflect.getOwnPropertyDescriptor(params, "customField");
    assert.sameValue(desc.value, 42);
    assert.sameValue(desc.writable, false);
    assert.sameValue(desc.enumerable, false);
    assert.sameValue(desc.configurable, true);
}
*/

// Escape
{
    const myURL = new URL('https://example.org/abc?fo~o=~ba r%z');

    assert.sameValue(myURL.search, "?fo~o=~ba%20r%z");

    // Modify the URL via searchParams...
    myURL.searchParams.sort();

    assert.sameValue(myURL.search, "?fo%7Eo=%7Eba+r%25z");
}
