# :material-decagram: Features

#### UI options

* Display realtime network speed in the notification

#### Service

SFA allows you to run sing-box through ForegroundService or VpnService (when TUN is required).

#### TUN

SFA provides an unprivileged TUN implementation through Android VpnService.

| TUN inbound option            | Available        | Note               |
|-------------------------------|------------------|--------------------|
| `interface_name`              | :material-close: | Managed by Android |
| `inet4_address`               | :material-check: | /                  |
| `inet6_address`               | :material-check: | /                  |
| `mtu`                         | :material-check: | /                  |
| `gso`                         | :material-close: | No permission      |
| `auto_route`                  | :material-check: | /                  |
| `strict_route`                | :material-close: | Not implemented    |
| `inet4_route_address`         | :material-check: | /                  |
| `inet6_route_address`         | :material-check: | /                  |
| `inet4_route_exclude_address` | :material-check: | /                  |
| `inet6_route_exclude_address` | :material-check: | /                  |
| `endpoint_independent_nat`    | :material-check: | /                  |
| `stack`                       | :material-check: | /                  |
| `include_interface`           | :material-close: | No permission      |
| `exclude_interface`           | :material-close: | No permission      |
| `include_uid`                 | :material-close: | No permission      |
| `exclude_uid`                 | :material-close: | No permission      |
| `include_android_user`        | :material-close: | No permission      |
| `include_package`             | :material-check: | /                  |
| `exclude_package`             | :material-check: | /                  |
| `platform`                    | :material-check: | /                  |

| Route/DNS rule option | Available        | Note                              |
|-----------------------|------------------|-----------------------------------|
| `process_name`        | :material-close: | No permission                     |
| `process_path`        | :material-close: | No permission                     |
| `process_path_regex`  | :material-close: | No permission                     |
| `package_name`        | :material-check: | /                                 |
| `user`                | :material-close: | Use `package_name` instead        |
| `user_id`             | :material-close: | Use `package_name` instead        |
| `wifi_ssid`           | :material-check: | Fine location permission required |
| `wifi_bssid`          | :material-check: | Fine location permission required |

### Override

Overrides profile configuration items with platform-specific values.

#### Per-app proxy

SFA allows you to select a list of Android apps that require proxying or bypassing in the graphical interface to
override the `include_package` and `exclude_package` configuration items.

In particular, the selector also provides the “China apps” scanning feature, providing Chinese users with an excellent
experience to bypass apps that do not require a proxy. Specifically, by scanning China application or SDK
characteristics through dex class path and other means, there will be almost no missed reports.

### Chore

* The working directory is located at `/sdcard/Android/data/io.nekohasekai.sfa/files` (External files directory)
* Crash logs is located in `$working_directory/stderr.log`
