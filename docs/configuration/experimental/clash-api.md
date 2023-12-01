---
icon: material/alert-decagram
---

!!! quote "Changes in sing-box 1.8.0"

    :material-delete-alert: [store_mode](#store_mode)  
    :material-delete-alert: [store_selected](#store_selected)  
    :material-delete-alert: [store_fakeip](#store_fakeip)  
    :material-delete-alert: [cache_file](#cache_file)  
    :material-delete-alert: [cache_id](#cache_id)


!!! quote ""

    Clash API is not included by default, see [Installation](./#installation).

### Structure

```json
{
  "external_controller": "127.0.0.1:9090",
  "external_ui": "",
  "external_ui_download_url": "",
  "external_ui_download_detour": "",
  "secret": "",
  "default_mode": "",
  
  // Deprecated
  
  "store_mode": false,
  "store_selected": false,
  "store_fakeip": false,
  "cache_file": "",
  "cache_id": ""
}
```

### Fields

#### external_controller

RESTful web API listening address. Clash API will be disabled if empty.

#### external_ui

A relative path to the configuration directory or an absolute path to a
directory in which you put some static web resource. sing-box will then
serve it at `http://{{external-controller}}/ui`.



#### external_ui_download_url

ZIP download URL for the external UI, will be used if the specified `external_ui` directory is empty.

`https://github.com/MetaCubeX/Yacd-meta/archive/gh-pages.zip` will be used if empty.

#### external_ui_download_detour

The tag of the outbound to download the external UI.

Default outbound will be used if empty.

#### secret

Secret for the RESTful API (optional)
Authenticate by spedifying HTTP header `Authorization: Bearer ${secret}`
ALWAYS set a secret if RESTful API is listening on 0.0.0.0

#### default_mode

Default mode in clash, `Rule` will be used if empty.

This setting has no direct effect, but can be used in routing and DNS rules via the `clash_mode` rule item.

#### store_mode

!!! failure "Deprecated in sing-box 1.8.0"

    `store_mode` is deprecated in Clash API and enabled by default if `cache_file.enabled`.

Store Clash mode in cache file.

#### store_selected

!!! failure "Deprecated in sing-box 1.8.0"

    `store_selected` is deprecated in Clash API and enabled by default if `cache_file.enabled`.

!!! note ""

    The tag must be set for target outbounds.

Store selected outbound for the `Selector` outbound in cache file.

#### store_fakeip

!!! failure "Deprecated in sing-box 1.8.0"

    `store_selected` is deprecated in Clash API and migrated to `cache_file.store_fakeip`.

Store fakeip in cache file.

#### cache_file

!!! failure "Deprecated in sing-box 1.8.0"

    `cache_file` is deprecated in Clash API and migrated to `cache_file.enabled` and `cache_file.path`.

Cache file path, `cache.db` will be used if empty.

#### cache_id

!!! failure "Deprecated in sing-box 1.8.0"

    `cache_id` is deprecated in Clash API and migrated to `cache_file.cache_id`.

Identifier in cache file.

If not empty, configuration specified data will use a separate store keyed by it.