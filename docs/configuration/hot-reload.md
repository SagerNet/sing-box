# Hot Reload Configuration

sing-box supports hot reloading of configurations to dynamically add, remove, or edit client connections without interrupting existing clients. This is especially useful for VPN servers managing many concurrent clients.

## Overview

Hot reload allows you to:
- Add new clients/peers without restarting the service
- Remove clients/peers while keeping others connected
- Update client settings (endpoints, allowed IPs, etc.)
- Minimize service disruption during configuration changes

## Triggering Hot Reload

### Method 1: Send SIGHUP Signal (Recommended)

On Unix-like systems (Linux, macOS, BSD):

```bash
# Find the sing-box process ID
ps aux | grep sing-box

# Send SIGHUP signal
kill -HUP <PID>

# Or use pkill
pkill -HUP sing-box

# Or use systemd
systemctl reload sing-box
```

### Method 2: Automatic File Watching (Future)

File watching is planned for a future release.

## What Can Be Hot Reloaded?

### ✅ Supported (No Restart Required)

#### WireGuard Endpoints
- Add/remove peers (clients)
- Update peer public keys
- Modify peer allowed IPs
- Change peer endpoints
- Adjust keepalive intervals
- Update pre-shared keys
- Modify reserved bytes

#### Hysteria2 Inbounds
- Add/remove users
- Update user passwords
- Change user names

#### VLESS Inbounds
- Add/remove users
- Update user UUIDs
- Change user flows
- Modify user names

#### Trojan Inbounds
- Add/remove users
- Update user passwords
- Change user names

#### VMess Inbounds
- Add/remove users
- Update user UUIDs
- Change user alterIds
- Modify user names

#### Inbounds & Outbounds
- Add new inbound/outbound entries
- Remove unused inbound/outbound entries
- Update configurations for protocols that support reload

### ❌ Requires Full Restart

- WireGuard private key changes
- WireGuard MTU modifications
- WireGuard listen port changes
- Log configuration
- Experimental features
- DNS server settings (coming soon)
- Route rules (coming soon)
- NTP settings

## Example: WireGuard Multi-Client Server

### Initial Configuration

```json
{
  "log": {
    "level": "info"
  },
  "endpoints": [
    {
      "type": "wireguard",
      "tag": "wg-server",
      "private_key": "YOUR_PRIVATE_KEY_BASE64",
      "address": ["10.0.0.1/24"],
      "listen_port": 51820,
      "peers": [
        {
          "public_key": "CLIENT1_PUBLIC_KEY",
          "allowed_ips": ["10.0.0.2/32"]
        },
        {
          "public_key": "CLIENT2_PUBLIC_KEY",
          "allowed_ips": ["10.0.0.3/32"]
        }
      ]
    }
  ],
  "route": {
    "rules": [
      {
        "inbound": "wg-server",
        "action": "route",
        "outbound": "direct"
      }
    ]
  }
}
```

### Adding a New Client (Hot Reload)

1. Edit your configuration file to add the new peer:

```json
{
  "endpoints": [
    {
      "type": "wireguard",
      "tag": "wg-server",
      "peers": [
        {
          "public_key": "CLIENT1_PUBLIC_KEY",
          "allowed_ips": ["10.0.0.2/32"]
        },
        {
          "public_key": "CLIENT2_PUBLIC_KEY",
          "allowed_ips": ["10.0.0.3/32"]
        },
        {
          "public_key": "CLIENT3_PUBLIC_KEY",
          "allowed_ips": ["10.0.0.4/32"]
        }
      ]
    }
  ]
}
```

2. Send SIGHUP to reload:

```bash
pkill -HUP sing-box
```

3. Check logs to verify hot reload succeeded:

```
[INFO] received SIGHUP, reloading configuration...
[INFO] reloading endpoint: wg-server
[INFO] adding WireGuard peer: CLIENT3_PUBLIC_...
[INFO] WireGuard peer reload completed successfully
[INFO] hot reload completed successfully
```

### Removing a Client (Hot Reload)

1. Remove the peer from your configuration
2. Send SIGHUP signal
3. The specified peer will be disconnected while others remain connected

### Updating a Client (Hot Reload)

1. Modify peer settings in configuration (e.g., allowed_ips)
2. Send SIGHUP signal
3. The peer configuration updates without disconnecting

## Example: Hysteria2 Multi-User Server

### Initial Configuration

```json
{
  "inbounds": [
    {
      "type": "hysteria2",
      "tag": "hy2-in",
      "listen": "::",
      "listen_port": 443,
      "users": [
        {
          "name": "alice",
          "password": "password1"
        },
        {
          "name": "bob",
          "password": "password2"
        }
      ],
      "tls": {
        "enabled": true,
        "server_name": "example.com",
        "certificate_path": "/path/to/cert.pem",
        "key_path": "/path/to/key.pem"
      }
    }
  ]
}
```

### Adding a New User (Hot Reload)

1. Edit configuration to add new user:

```json
{
  "inbounds": [
    {
      "type": "hysteria2",
      "users": [
        {
          "name": "alice",
          "password": "password1"
        },
        {
          "name": "bob",
          "password": "password2"
        },
        {
          "name": "charlie",
          "password": "password3"
        }
      ]
    }
  ]
}
```

2. Send SIGHUP to reload:

```bash
pkill -HUP sing-box
```

3. Check logs:

```
[INFO] received SIGHUP, reloading configuration...
[INFO] reloading inbound: hy2-in
[INFO] performing hot reload of Hysteria2 users
[INFO] Hysteria2 user reload completed successfully, 3 users configured
[INFO] hot reload completed successfully
```

## Example: VLESS Multi-User Server

### Initial Configuration

```json
{
  "inbounds": [
    {
      "type": "vless",
      "tag": "vless-in",
      "listen": "::",
      "listen_port": 443,
      "users": [
        {
          "name": "user1",
          "uuid": "5783a3e7-e1b0-4deb-9418-5f6e86b8b8c4",
          "flow": ""
        },
        {
          "name": "user2",
          "uuid": "d5cd0e8b-5af5-4e4e-b5a9-1f6c7c7e0c8a",
          "flow": ""
        }
      ],
      "tls": {
        "enabled": true,
        "server_name": "example.com",
        "certificate_path": "/path/to/cert.pem",
        "key_path": "/path/to/key.pem"
      }
    }
  ]
}
```

### Adding/Updating Users (Hot Reload)

1. Edit configuration:

```json
{
  "inbounds": [
    {
      "type": "vless",
      "users": [
        {
          "name": "user1",
          "uuid": "5783a3e7-e1b0-4deb-9418-5f6e86b8b8c4",
          "flow": ""
        },
        {
          "name": "user2",
          "uuid": "d5cd0e8b-5af5-4e4e-b5a9-1f6c7c7e0c8a",
          "flow": "xtls-rprx-vision"
        },
        {
          "name": "user3",
          "uuid": "a1b2c3d4-e5f6-4e4e-b5a9-0f1a2b3c4d5e",
          "flow": ""
        }
      ]
    }
  ]
}
```

2. Send SIGHUP:

```bash
pkill -HUP sing-box
```

3. Verify in logs:

```
[INFO] received SIGHUP, reloading configuration...
[INFO] reloading inbound: vless-in
[INFO] performing hot reload of VLESS users
[INFO] VLESS user reload completed successfully, 3 users configured
[INFO] hot reload completed successfully
```

## Example: Trojan Multi-User Server

### Initial Configuration

```json
{
  "inbounds": [
    {
      "type": "trojan",
      "tag": "trojan-in",
      "listen": "::",
      "listen_port": 443,
      "users": [
        {
          "name": "alice",
          "password": "password1"
        },
        {
          "name": "bob",
          "password": "password2"
        }
      ],
      "tls": {
        "enabled": true,
        "server_name": "example.com",
        "certificate_path": "/path/to/cert.pem",
        "key_path": "/path/to/key.pem"
      }
    }
  ]
}
```

### Adding/Updating Users (Hot Reload)

1. Edit configuration:

```json
{
  "inbounds": [
    {
      "type": "trojan",
      "users": [
        {
          "name": "alice",
          "password": "new_password1"
        },
        {
          "name": "bob",
          "password": "password2"
        },
        {
          "name": "charlie",
          "password": "password3"
        }
      ]
    }
  ]
}
```

2. Send SIGHUP:

```bash
pkill -HUP sing-box
```

3. Verify in logs:

```
[INFO] received SIGHUP, reloading configuration...
[INFO] reloading inbound: trojan-in
[INFO] performing hot reload of Trojan users
[INFO] Trojan user reload completed successfully, 3 users configured
[INFO] hot reload completed successfully
```

## Example: VMess Multi-User Server

### Initial Configuration

```json
{
  "inbounds": [
    {
      "type": "vmess",
      "tag": "vmess-in",
      "listen": "::",
      "listen_port": 8080,
      "users": [
        {
          "name": "alice",
          "uuid": "550e8400-e29b-41d4-a716-446655440000",
          "alterId": 0
        },
        {
          "name": "bob",
          "uuid": "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
          "alterId": 0
        }
      ]
    }
  ]
}
```

### Adding/Updating Users (Hot Reload)

1. Edit configuration:

```json
{
  "inbounds": [
    {
      "type": "vmess",
      "users": [
        {
          "name": "alice",
          "uuid": "550e8400-e29b-41d4-a716-446655440000",
          "alterId": 0
        },
        {
          "name": "bob",
          "uuid": "new-uuid-for-bob-here-00000000000",
          "alterId": 0
        },
        {
          "name": "charlie",
          "uuid": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
          "alterId": 0
        }
      ]
    }
  ]
}
```

2. Send SIGHUP:

```bash
pkill -HUP sing-box
```

3. Verify in logs:

```
[INFO] received SIGHUP, reloading configuration...
[INFO] reloading inbound: vmess-in
[INFO] performing hot reload of VMess users
[INFO] VMess user reload completed successfully, 3 users configured
[INFO] hot reload completed successfully
```

## Behavior During Hot Reload

### Existing Connections
- Continue using old configuration
- Not interrupted or disconnected
- Close naturally when client disconnects

### New Connections
- Use new configuration immediately
- Connect with updated settings
- No delay or downtime

### Failed Reload
If hot reload fails for any reason:
1. Current configuration remains active
2. Error is logged
3. Service automatically falls back to full restart if critical
4. Existing connections may be interrupted only on full restart

## Monitoring Hot Reload

### Check Logs

```bash
# Watch logs in real-time
journalctl -u sing-box -f

# Check recent reload attempts
journalctl -u sing-box | grep reload
```

### Example Log Output

**Successful Hot Reload:**
```
[INFO] received SIGHUP, reloading configuration...
[INFO] reloading endpoint: wg-server
[INFO] adding WireGuard peer: abc123...
[INFO] updating WireGuard peer: def456...
[INFO] removing WireGuard peer: ghi789...
[INFO] WireGuard peer reload completed successfully
[INFO] hot reload completed successfully
```

**Failed Hot Reload (Falls Back to Restart):**
```
[INFO] received SIGHUP, reloading configuration...
[ERROR] config validation failed: invalid peer public key
[ERROR] keeping current configuration
```

## Best Practices

1. **Test Configuration First**
   ```bash
   sing-box check -c config.json
   ```

2. **Backup Current Config**
   ```bash
   cp config.json config.json.backup
   ```

3. **Make Incremental Changes**
   - Add/remove one client at a time for easier troubleshooting
   - Test each change before applying more

4. **Monitor Logs**
   - Always check logs after reload
   - Verify expected peers are added/removed

5. **Automate with Scripts**
   ```bash
   #!/bin/bash
   # add-wireguard-peer.sh
   
   # Validate config
   if ! sing-box check -c /etc/sing-box/config.json; then
       echo "Invalid configuration"
       exit 1
   fi
   
   # Reload
   pkill -HUP sing-box
   
   # Check result
   sleep 1
   journalctl -u sing-box -n 20
   ```

## Limitations

- Some protocols don't support hot reload yet (only WireGuard initially)
- Critical setting changes require full restart
- Very large peer lists (>1000 peers) may take a few seconds to reload

## Future Enhancements

Planned features:
- Automatic file watching and reload
- API endpoint for programmatic reload
- Support for more protocols (Shadowsocks, Trojan, VLESS, VMess)
- DNS and routing rule hot reload
- Gradual client migration during updates

## Troubleshooting

### Reload Not Working

1. Check if sing-box is running:
   ```bash
   systemctl status sing-box
   ```

2. Verify SIGHUP was received:
   ```bash
   journalctl -u sing-box | grep SIGHUP
   ```

3. Check for config errors:
   ```bash
   sing-box check -c /etc/sing-box/config.json
   ```

### Clients Disconnected After Reload

If clients disconnect, it usually means:
- A critical setting changed (requires full restart)
- Configuration error caused fallback to restart
- Check logs for the specific reason

### Reload Takes Too Long

For very large peer lists:
- Normal for >500 peers to take several seconds
- Consider splitting into multiple endpoints
- Monitor system resources during reload

## See Also

- [WireGuard Configuration](./wireguard.md)
- [Endpoint Configuration](./endpoint.md)
- [Route Rules](./route.md)

