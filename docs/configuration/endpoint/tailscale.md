---
icon: material/new-box
---

!!! question "Since sing-box 1.12.0"

### Structure

```json
{
  "type": "tailscale",
  "tag": "ts-ep",
  "state_directory": "",
  "auth_key": "",
  "control_url": "",
  "ephemeral": false,
  "hostname": "",
  "accept_routes": false,
  "exit_node": "",
  "exit_node_allow_lan_access": false,
  "advertise_routes": [],
  "advertise_exit_node": false,
  "udp_timeout": "5m",
  
  ... // Dial Fields
}
```

### Fields

#### state_directory

The directory where the Tailscale state is stored.

`tailscale` is used by default.

Example: `$HOME/.tailscale`

#### auth_key

!!! note
    
    Auth key is not required. By default, sing-box will log the login URL (or popup a notification on graphical clients).

The auth key to create the node. If the node is already created (from state previously stored), then this field is not
used.

#### control_url

The coordination server URL.

`https://controlplane.tailscale.com` is used by default.

#### ephemeral

Indicates whether the instance should register as an Ephemeral node (https://tailscale.com/s/ephemeral-nodes).

#### hostname

The hostname of the node.

System hostname is used by default.

Example: `localhost`

#### accept_routes

Indicates whether the node should accept routes advertised by other nodes.

#### exit_node

The exit node name or IP address to use.

#### exit_node_allow_lan_access

!!! note

    When the exit node does not have a corresponding advertised route, private traffics cannot be routed to the exit node even if `exit_node_allow_lan_access is` set.

Indicates whether locally accessible subnets should be routed directly or via the exit node.

#### advertise_routes

CIDR prefixes to advertise into the Tailscale network as reachable through the current node.

Example: `["192.168.1.1/24"]`

#### advertise_exit_node

Indicates whether the node should advertise itself as an exit node.

#### udp_timeout

UDP NAT expiration time.

`5m` will be used by default.

### Dial Fields

!!! note

    Dial Fields in Tailscale endpoints only control how it connects to the control plane and have nothing to do with actual connections.

See [Dial Fields](/configuration/shared/dial/) for details.
