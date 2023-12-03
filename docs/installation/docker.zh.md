---
icon: material/docker
---

# Docker

## :material-console: 命令

```bash
docker run -d \
  -v /etc/sing-box:/etc/sing-box/ \
  --name=sing-box \
  --restart=always \
  ghcr.io/sagernet/sing-box \
  -D /var/lib/sing-box \
  -C /etc/sing-box/ run
```

## :material-box-shadow: Compose

```yaml
version: "3.8"
services:
  sing-box:
    image: ghcr.io/sagernet/sing-box
    container_name: sing-box
    restart: always
    volumes:
      - /etc/sing-box:/etc/sing-box/
    command: -D /var/lib/sing-box -C /etc/sing-box/ run
```
