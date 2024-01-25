---
icon: material/lightning-bolt
---

# Hysteria 2

Hysteria 2 is a simple, Chinese-made protocol based on QUIC.
The selling point is Brutal, a congestion control algorithm that
tries to achieve a user-defined bandwidth despite packet loss.

!!! warning

    Even though GFW rarely blocks UDP-based proxies, such protocols actually have far more obvious characteristics than TCP based proxies.

| Specification                                                             | Resists passive detection | Resists active probes |
|---------------------------------------------------------------------------|---------------------------|-----------------------|
| [hysteria.network](https://v2.hysteria.network/docs/developers/Protocol/) | :material-alert:          | :material-check:      |

## :material-text-box-check: Password Generator

| Generate Password          | Action                                                          |
|----------------------------|-----------------------------------------------------------------|
| <code id="password"><code> | <button class="md-button" onclick="generate()">Refresh</button> |

<script>
    function generate() {
        const array = new Uint8Array(16);
        window.crypto.getRandomValues(array);
        document.getElementById("password").textContent = btoa(String.fromCharCode.apply(null, array));
    }
    generate();
</script>

## :material-alert: Difference from official Hysteria

The official program supports an authentication method called **userpass**,
which essentially uses a combination of `<username>:<password>` as the actual password,
while sing-box does not provide this alias.
To use sing-box with the official program, you need to fill in that combination as the actual password.

## :material-server: Server Example

!!! info ""

    Replace `up_mbps` and `down_mbps` values with the actual bandwidth of your server.

=== ":material-harddisk: With local certificate"

    ```json
     {
      "inbounds": [
        {
          "type": "hysteria2",
          "listen": "::",
          "listen_port": 8080,
          "up_mbps": 100,
          "down_mbps": 100,
          "users": [
            {
              "name": "sekai",
              "password": "<password>"
            }
          ],
          "tls": {
            "enabled": true,
            "server_name": "example.org",
            "key_path": "/path/to/key.pem",
            "certificate_path": "/path/to/certificate.pem"
          }
        }
      ]
    }
    ```

=== ":material-auto-fix: With ACME"

    ```json
     {
      "inbounds": [
        {
          "type": "hysteria2",
          "listen": "::",
          "listen_port": 8080,
          "up_mbps": 100,
          "down_mbps": 100,
          "users": [
            {
              "name": "sekai",
              "password": "<password>"
            }
          ],
          "tls": {
            "enabled": true,
            "server_name": "example.org",
            "acme": {
              "domain": "example.org",
              "email": "admin@example.org"
            }
          }
        }
      ]
    }
    ```

=== ":material-cloud: With ACME and Cloudflare API"

    ```json
     {
      "inbounds": [
        {
          "type": "hysteria2",
          "listen": "::",
          "listen_port": 8080,
          "up_mbps": 100,
          "down_mbps": 100,
          "users": [
            {
              "name": "sekai",
              "password": "<password>"
            }
          ],
          "tls": {
            "enabled": true,
            "server_name": "example.org",
            "acme": {
              "domain": "example.org",
              "email": "admin@example.org",
              "dns01_challenge": {
                "provider": "cloudflare",
                "api_token": "my_token"
              }
            }
          }
        }
      ]
    }
    ```

## :material-cellphone-link: Client Example

!!! info ""

    Replace `up_mbps` and `down_mbps` values with the actual bandwidth of your client.

=== ":material-web-check: With valid certificate"

    ```json
    {
      "outbounds": [
        {
          "type": "hysteria2",
          "server": "127.0.0.1",
          "server_port": 8080,
          "up_mbps": 100,
          "down_mbps": 100,
          "password": "<password>",
          "tls": {
            "enabled": true,
            "server_name": "example.org"
          }
        }
      ]
    }
    ```

=== ":material-check: With self-sign certificate"

    !!! info "Tip"
        
        Use `sing-box merge` command to merge configuration and certificate into one file.

    ```json
    {
      "outbounds": [
        {
          "type": "hysteria2",
          "server": "127.0.0.1",
          "server_port": 8080,
          "up_mbps": 100,
          "down_mbps": 100,
          "password": "<password>",
          "tls": {
            "enabled": true,
            "server_name": "example.org",
            "certificate_path": "/path/to/certificate.pem"
          }
        }
      ]
    }
    ```

=== ":material-alert: Ignore certificate verification"

    ```json
    {
      "outbounds": [
        {
          "type": "hysteria2",
          "server": "127.0.0.1",
          "server_port": 8080,
          "up_mbps": 100,
          "down_mbps": 100,
          "password": "<password>",
          "tls": {
            "enabled": true,
            "server_name": "example.org",
            "insecure": true
          }
        }
      ]
    }
    ```
