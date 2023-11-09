# :material-decagram: Features

#### UI options

* Display realtime network speed in the notification

#### Service

SFA allows you to run sing-box through ForegroundService or VpnService (when TUN is required).

#### TUN

SFA provides an unprivileged TUN implementation through Android VpnService.

| TUN inbound option            | Available | Note               |
|-------------------------------|-----------|--------------------|
| `interface_name`              | ✖️        | Managed by Android |
| `inet4_address`               | ✔️        | /                  |
| `inet6_address`               | ✔️        | /                  |
| `mtu`                         | ✔️        | /                  |
| `auto_route`                  | ✔️        | /                  |
| `strict_route`                | ✖️        | Not implemented    |
| `inet4_route_address`         | ✔️        | /                  |
| `inet6_route_address`         | ✔️        | /                  |
| `inet4_route_exclude_address` | ✔️        | /                  |
| `inet6_route_exclude_address` | ✔️        | /                  |
| `endpoint_independent_nat`    | ✔️        | /                  |
| `stack`                       | ✔️        | /                  |
| `include_interface`           | ✖️        | No permission      |
| `exclude_interface`           | ✖️        | No permission      |
| `include_uid`                 | ✖️        | No permission      |
| `exclude_uid`                 | ✖️        | No permission      |
| `include_android_user`        | ✖️        | No permission      |
| `include_package`             | ✔️        | /                  |
| `exclude_package`             | ✔️        | /                  |
| `platform`                    | ✔️        | /                  |

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
