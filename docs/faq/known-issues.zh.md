#### DNS

##### macOS

`auto-route` 无法自动劫持发往局域网的 DNS 请求，需要手动设置位于公网的 DNS 服务器。

##### Android

`auto-route` 无法自动劫持 DNS 请求如果 `私人 DNS` 开启.

##### Linux

`auto-route` 无法自动劫持 DNS 请求如果 `systemd-resoled` 开启, 您可以切换到 NetworkManager.

#### 系统代理

##### Linux

通常只有浏览器和 GNOME 应用程序接受 GNOME 代理设置。

##### Android

启用系统代理后，某些应用程序会出错（通常来自中国）。
