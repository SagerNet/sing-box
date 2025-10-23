> Sponsored by [Warp](https://go.warp.dev/sing-box), built for coding with multiple AI agents

<a href="https://go.warp.dev/sing-box">
<img alt="Warp sponsorship" width="400" src="https://github.com/warpdotdev/brand-assets/raw/refs/heads/main/Github/Sponsor/Warp-Github-LG-02.png">
</a>

---

# sing-box

The universal proxy platform.

[![Packaging status](https://repology.org/badge/vertical-allrepos/sing-box.svg)](https://repology.org/project/sing-box/versions)

## Documentation

https://sing-box.sagernet.org

## Hot Reload

sing-box supports hot reloading of configuration without service interruption. Supported protocols: WireGuard, Hysteria2, VLESS, Trojan, VMess.

### Binary Usage

Edit your configuration file and send SIGHUP signal:

```bash
# Edit configuration
vim /etc/sing-box/config.json

# Trigger hot reload
pkill -HUP sing-box

# Or using systemd
systemctl reload sing-box
```

Existing connections continue uninterrupted while new connections use the updated configuration.

**Supported hot reload changes:**
- WireGuard: Add/remove/update peers
- Hysteria2, Trojan: Add/remove/update users and passwords
- VLESS: Add/remove/update users, UUIDs, and flows
- VMess: Add/remove/update users, UUIDs, and alterIds

**Changes requiring full restart:**
- Listen address/port changes
- TLS certificate paths
- Transport type changes
- DNS/routing rule changes

### Golang Library Usage

```go
package main

import (
    "context"
    "github.com/sagernet/sing-box"
    "github.com/sagernet/sing-box/option"
)

func main() {
    // Create instance
    ctx := context.Background()
    options, _ := option.ReadConfigFile("config.json")
    instance, _ := box.New(box.Options{
        Context: ctx,
        Options: options,
    })
    
    instance.Start()
    
    // Hot reload with new configuration
    newOptions, _ := option.ReadConfigFile("config.json")
    err := instance.Reload(newOptions)
    if err != nil {
        // Reload failed, instance continues with old config
        log.Error("Hot reload failed: ", err)
    }
    
    instance.Close()
}
```

For more details, see [Hot Reload Documentation](https://sing-box.sagernet.org/configuration/hot-reload/).

## License

```
Copyright (C) 2022 by nekohasekai <contact-sagernet@sekai.icu>

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program. If not, see <http://www.gnu.org/licenses/>.

In addition, no derivative work may use the name or imply association
with this application without prior consent.
```