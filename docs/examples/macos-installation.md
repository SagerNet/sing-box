#### Download sing-box

Download sing-box macOS binary package from [GitHub Releases](https://github.com/SagerNet/sing-box/releases/latest) or Github Actions. Move the `sing-box` to the appropriate location (e.g. /Applications/sing-box/, hereafter referred to as the `working directory`).

#### Creates Plist file

Create a new Plist file and save to ~/Library/LaunchAgents/

Example:

```bash
nano ~/Library/LaunchAgents/org.sagernet.sing-box.plist
```

The content is as follows (`[]` needs to be deleted) :

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
        <string>/Applications/sing-box/sing-box [Path to sing-box]</string>
        <string>run</string>
        <string>--config</string>
        <string>/Applications/sing-box/config.json [Path to config.json]</string>
        <string>--directory</string>
        <string>/Applications/sing-box [Path to working directory]</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
</dict>
</plist>
```

#### Check the Plist file

```bash
plutil ~/Library/LaunchAgents/org.sagernet.sing-box.plist
```

#### Load the plist file

```bash
launchctl load ~/Library/LaunchAgents/org.sagernet.sing-box.plist
```

#### Confirm

```bash
launchctl list | grep org.sagernet.sing-box
```

After the correct configuration, sing-box will be load after booting automatically.

To stop the Sing-box service, replace the command from `load` to `unload`.
