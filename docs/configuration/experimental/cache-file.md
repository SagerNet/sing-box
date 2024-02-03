!!! question "Since sing-box 1.8.0"

!!! quote "Changes in sing-box 1.9.0"

    :material-plus: [store_rdrc](#store_rdrc)  
    :material-plus: [rdrc_timeout](#rdrc_timeout)  

### Structure

```json
{
  "enabled": true,
  "path": "",
  "cache_id": "",
  "store_fakeip": false,
  "store_rdrc": false,
  "rdrc_timeout": ""
}
```

### Fields

#### enabled

Enable cache file.

#### path

Path to the cache file.

`cache.db` will be used if empty.

#### cache_id

Identifier in the cache file

If not empty, configuration specified data will use a separate store keyed by it.

#### store_fakeip

Store fakeip in the cache file

#### store_rdrc

Store rejected DNS response cache in the cache file

The check results of [Address filter DNS rule items](/configuration/dns/rule/#address-filter-fields)
will be cached until expiration.

#### rdrc_timeout

Timeout of rejected DNS response cache.

`7d` is used by default.
