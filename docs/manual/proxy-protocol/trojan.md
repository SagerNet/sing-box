---
icon: material/horse
---

# Trojan

Torjan is the most commonly used TLS proxy made in China. It can be used in various combinations,
but only the combination of uTLS and multiplexing is recommended.

| Protocol and implementation combination | Specification                                                        | Resists passive detection | Resists active probes |
|-----------------------------------------|----------------------------------------------------------------------|---------------------------|-----------------------|
| Origin / trojan-gfw                     | [trojan-gfw.github.io](https://trojan-gfw.github.io/trojan/protocol) | :material-check:          | :material-check:      |
| Basic Go implementation                 | /                                                                    | :material-alert:          | :material-check:      |
| with privates transport by V2Ray        | No formal definition                                                 | :material-alert:          | :material-alert:      |
| with uTLS enabled                       | No formal definition                                                 | :material-help:           | :material-check:      |

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

## :material-server: Server Example

=== ":material-harddisk: With local certificate"

    ```json
    {
      "inbounds": [
        {
          "type": "trojan",
          "listen": "::",
          "listen_port": 8080,
          "users": [
            {
              "name": "example",
              "password": "password"
            }
          ],
          "tls": {
            "enabled": true,
            "server_name": "example.org",
            "key_path": "/path/to/key.pem",
            "certificate_path": "/path/to/certificate.pem"
          },
          "multiplex": {
            "enabled": true
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
          "type": "trojan",
          "listen": "::",
          "listen_port": 8080,
          "users": [
            {
              "name": "example",
              "password": "password"
            }
          ],
          "tls": {
            "enabled": true,
            "server_name": "example.org",
            "acme": {
              "domain": "example.org",
              "email": "admin@example.org"
            }
          },
          "multiplex": {
            "enabled": true
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
          "type": "trojan",
          "listen": "::",
          "listen_port": 8080,
          "users": [
            {
              "name": "example",
              "password": "password"
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
          },
          "multiplex": {
            "enabled": true
          }
        }
      ]
    }
    ```

## :material-cellphone-link: Client Example

=== ":material-web-check: With valid certificate"

    ```json
    {
      "outbounds": [
        {
          "type": "trojan",
          "server": "127.0.0.1",
          "server_port": 8080,
          "password": "password",
          "tls": {
            "enabled": true,
            "server_name": "example.org",
            "utls": {
              "enabled": true,
              "fingerprint": "firefox"
            }
          },
          "multiplex": {
            "enabled": true
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
          "type": "trojan",
          "server": "127.0.0.1",
          "server_port": 8080,
          "password": "password",
          "tls": {
            "enabled": true,
            "server_name": "example.org",
            "certificate_path": "/path/to/certificate.pem",
            "utls": {
              "enabled": true,
              "fingerprint": "firefox"
            }
          },
          "multiplex": {
            "enabled": true
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
          "type": "trojan",
          "server": "127.0.0.1",
          "server_port": 8080,
          "password": "password",
          "tls": {
            "enabled": true,
            "server_name": "example.org",
            "insecure": true,
            "utls": {
              "enabled": true,
              "fingerprint": "firefox"
            }
          },
          "multiplex": {
            "enabled": true
          }
        }
      ]
    }
    ```
