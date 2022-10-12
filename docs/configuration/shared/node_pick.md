### Structure

```json
{
  "expected": 1,
  "baselines": [
    "50ms",
    "100ms",
    "150ms",
    "200ms",
    "250ms",
    "350ms"
  ],
  "costs": [
    {
      "match": "proxy-c",
      "value": 10
    },
    {
      "match": "x2.0"
    },
    {
      "regexp": true,
      "match": "x\\d+(\\.\\d+)?"
    }
  ]
}
```

### expected

The expected number of outbound to be selected. The default value is 1.


### baselines

The RTT baselines which divides the node into different ranges. The default value is empty.

!!! tip  "How does expected and baselines works"

    The following examples illustrate the logic of different configuration combinations:

    1. expected=`3` baselines =`[]`, select 3 nodes with the smallest RTT in recent checks.

    1. expected=`3` baselines =`["50ms", "100ms", "150ms"]`。
    
        Suppose in the previous example, 3 nodes of `40ms`, `65ms`, `90ms` are selected, but there are more nodes of `90-100ms`, which are almost as good as the selected ones, we do not hope to waste them.
    
        With the above baselines, to select 3 nodes, it must step into the `50-100ms` range, then other nodes in this range are also selected.

    > The RTT described above, for the `leastload` strategy, refers to the node cost-weighted RTT standard deviation; for the `leastping` strategy, it refers to the node cost-weighted RTT average.

### costs

The cost rules of outbounds. The default value is empty.

Each cost rule has three fields:

```json
{
  "regexp": true,
  "match": "x\\d+(\\.\\d+)?",
  "value": 10
}
```

It sets the cost of a outbound by matching its tag. The cost value will be eventually weighted to the node's RTT. The effect is that the higher the cost of a node, the lower the probability of it being selected.

The preceding cost rule has higher priority: if multiple rules can be matched, only the first matched rule will be used.

If no rule is matched, the node has a cost of `1`.

### costs.regexp

To enable regular expression matching. The default value is `false`.

### costs.match

match pattern.

### costs.value

Explicitly set the cost value for matching nodes. Defaults to `0`, i.e. not explicitly set.

If the field is not explicit set, the node's cost value is automatically set based on the number matched in the `match` expression from the tag. For example, matching the node `proxy-b-x2.0` with the regular expression `x\\d+(\\.\\d+)?` gets `x2.0`, then the number `2.0` is extracted and set to the cost value of this node.

If not explicitly set, and no number is found in the matched text, the cost value is set to `1`.
