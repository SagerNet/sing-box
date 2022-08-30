#### 下载 sing-box

从 [GitHub Releases](https://github.com/SagerNet/sing-box/releases/latest) 或 GitHub Actions 下载适用于 macOS 的压缩包，解压后将 `sing-box` 文件复制到合适的位置（例如：`/Applications/sing-box/`，后文简称为`工作目录`）。

#### 建立服务文件

新建服务文件并保存到 ~/Library/LaunchAgents/

示例：

```bash
nano ~/Library/LaunchAgents/org.sagernet.sing-box.plist
```

内容如下（`【】`需要删除）：

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>KeepAlive</key>
    <true/>
    <key>Label</key>
    <string>org.sagernet.sing-box</string>
    <key>ProgramArguments</key>
    <array>
        <string>/Applications/sing-box/sing-box【可执行文件路径】</string>
        <string>run</string>
        <string>--config</string>
        <string>/Applications/sing-box/config.json【配置文件路径】</string>
        <string>--directory</string>
        <string>/Applications/sing-box【工作目录】</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
</dict>
</plist>
```

#### 检查文件是否正确

```bash
plutil ~/Library/LaunchAgents/org.sagernet.sing-box.plist
```

#### 加载plist文件

```bash
launchctl load ~/Library/LaunchAgents/org.sagernet.sing-box.plist
```

#### 确认加载

```bash
launchctl list | grep org.sagernet.sing-box
```

正确配置后 sing-box 将开机自启动。

如果需要关闭 sing-box 服务，将上述命令从 `load` 替换为 `unload` 即可。
