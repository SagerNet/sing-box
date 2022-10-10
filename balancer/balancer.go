package balancer

// Balancer is interface for load balancers
type Balancer interface {
	Select() *Node
}
