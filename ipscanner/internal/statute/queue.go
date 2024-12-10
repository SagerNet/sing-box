package statute

import (
	"sort"
	"time"
)

type IPInfQueue struct {
	items []IPInfo
}

// Enqueue adds an item and then sorts the queue.
func (q *IPInfQueue) Enqueue(item IPInfo) {
	q.items = append(q.items, item)
	sort.Slice(q.items, func(i, j int) bool {
		return q.items[i].RTT < q.items[j].RTT
	})
}

// Dequeue removes and returns the item with the lowest RTT.
func (q *IPInfQueue) Dequeue() IPInfo {
	if len(q.items) == 0 {
		return IPInfo{} // Returning an empty IPInfo when the queue is empty.
	}
	item := q.items[0]
	q.items = q.items[1:]
	item.CreatedAt = time.Now()
	return item
}

// Size returns the number of items in the queue.
func (q *IPInfQueue) Size() int {
	return len(q.items)
}
