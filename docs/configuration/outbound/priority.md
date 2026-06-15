### Structure

```json
{
  "type": "priority",
  "tag": "priority",

  "tiers": [
    ["proxy-a", "proxy-b"],
    ["proxy-c"],
    ["proxy-d"]
  ],
  "url": "",
  "interval": "",
  "tolerance": 0,
  "idle_timeout": "",
  "tier_up_checks": 0,
  "tier_down_checks": 0,
  "interrupt_exist_connections": false
}
```

A priority group holds the highest-priority tier that has at least one healthy
member, drops to a lower tier only when the active tier has fully failed, and
climbs back up once a higher tier has recovered. Within the active tier it
behaves like [URLTest](./urltest.md), selecting the lowest-delay member.

### Fields

#### tiers

==Required==

List of tiers, ordered from highest to lowest priority. Each tier is a list of
outbound tags. The first tier is preferred; lower tiers are used only as
fallback. Making the last tier a catch-all (for example, every available exit)
keeps the group from going dark when every preferred tier is down.

#### url

The URL to test. `https://www.gstatic.com/generate_204` will be used if empty.

#### interval

The test interval. `3m` will be used if empty.

#### tolerance

The test tolerance in milliseconds, applied when selecting between members of
the active tier. `50` will be used if empty.

#### idle_timeout

The idle timeout. `30m` will be used if empty.

#### tier_up_checks

Number of consecutive healthy probe rounds a higher-priority tier must pass
before the group climbs back up to it. `2` will be used if empty. Raise it to
dampen flapping when a preferred tier is unstable.

#### tier_down_checks

Number of consecutive failed probe rounds the active tier must register before
the group drops to a lower tier. `1` will be used if empty (drop on the first
fully-failed round).

#### interrupt_exist_connections

Interrupt existing connections when the selected outbound has changed.

Only inbound connections are affected by this setting, internal connections will always be interrupted.
