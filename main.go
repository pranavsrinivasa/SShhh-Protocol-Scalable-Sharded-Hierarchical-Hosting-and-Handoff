package main

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// Request represents a task or data query.
type Request struct {
	Data   string
	Region string // Region for collaborative filtering (e.g., North, South)
	From   int    // Originating node ID (0 means from external source)
}

// CentralServer holds the initial data and manages replication.
type CentralServer struct {
	Data              map[string]string      // dataID -> data content
	ReplicationFactor int                    // How many nodes should replicate each item
	ReplicationMap    map[string][]int       // dataID -> list of node IDs that hold the data
	Mutex             sync.Mutex
}

// OffloadDataToNode replicates data items from the central server to a new node
// if they haven't met the desired replication factor.
func (cs *CentralServer) OffloadDataToNode(node *Node) {
	cs.Mutex.Lock()
	defer cs.Mutex.Unlock()
	for key, value := range cs.Data {
		// Check current replication count for this item.
		nodesHolding, exists := cs.ReplicationMap[key]
		if !exists || len(nodesHolding) < cs.ReplicationFactor {
			// Offload this data item to the node.
			node.Mutex.Lock()
			node.Data[key] = value
			node.Mutex.Unlock()
			// Update replication map.
			cs.ReplicationMap[key] = append(cs.ReplicationMap[key], node.ID)
			fmt.Printf("CentralServer: Offloaded data item '%s' to Node %d\n", key, node.ID)
		}
	}
}

// Node represents a device participating in the SHhh network.
type Node struct {
	ID      int
	Region  string
	Load    int // Simplified load count
	Data    map[string]string
	Channel chan Request
	Manager *NodeManager
	Mutex   sync.Mutex
}

// Start begins the node's event loop to process incoming requests.
func (n *Node) Start(wg *sync.WaitGroup) {
	defer wg.Done()
	for req := range n.Channel {
		n.Mutex.Lock()
		n.Load++ // Increase load upon receiving a request
		n.Mutex.Unlock()
		fmt.Printf("Node %d (Region: %s, Load: %d) processing: %s\n", n.ID, n.Region, n.Load, req.Data)
		// Simulate processing delay.
		time.Sleep(time.Duration(rand.Intn(100)+50) * time.Millisecond)

		// If overloaded, attempt to offload this request to a less busy node.
		n.Mutex.Lock()
		if n.Load > 3 {
			fmt.Printf("Node %d overloaded. Trying to offload '%s'\n", n.ID, req.Data)
			n.Manager.OffloadRequest(req, n.ID)
			n.Load-- // Reduce local load as we handed off the task.
		}
		n.Mutex.Unlock()
	}
}

// NodeManager manages node registration and intelligent routing.
type NodeManager struct {
	Nodes         []*Node
	Mutex         sync.Mutex
	CentralServer *CentralServer
}

// RegisterNode adds a node to the system and offloads data from the central server.
func (nm *NodeManager) RegisterNode(n *Node) {
	nm.Mutex.Lock()
	defer nm.Mutex.Unlock()
	nm.Nodes = append(nm.Nodes, n)
	// Offload data from the central server onto the new node.
	nm.CentralServer.OffloadDataToNode(n)
}

// CollaborativeScore returns a simple score based on region matching.
// A higher score means a better candidate for offloading a request.
func (nm *NodeManager) CollaborativeScore(node *Node, req Request) float64 {
	if node.Region == req.Region {
		return 1.0
	}
	return 0.5
}

// FindOptimalNode selects the best candidate node for handling the request.
// It uses a score that factors in collaborative filtering and current load.
func (nm *NodeManager) FindOptimalNode(req Request, excludeID int) *Node {
	nm.Mutex.Lock()
	defer nm.Mutex.Unlock()
	var bestNode *Node
	bestScore := -1.0
	for _, node := range nm.Nodes {
		if node.ID == excludeID {
			continue
		}
		// Only consider nodes in the same region (or you can relax this rule).
		if node.Region != req.Region {
			continue
		}
		node.Mutex.Lock()
		// Calculate a simple routing score.
		score := nm.CollaborativeScore(node, req) / float64(node.Load+1)
		node.Mutex.Unlock()
		if score > bestScore {
			bestScore = score
			bestNode = node
		}
	}
	return bestNode
}

// OffloadRequest attempts to hand off a request from an overloaded node
// to an optimal candidate node.
func (nm *NodeManager) OffloadRequest(req Request, fromID int) {
	candidate := nm.FindOptimalNode(req, fromID)
	if candidate != nil {
		fmt.Printf("NodeManager: Offloading '%s' from Node %d to Node %d\n", req.Data, fromID, candidate.ID)
		candidate.Channel <- req
	} else {
		fmt.Printf("NodeManager: No optimal candidate found for offloading '%s' from Node %d. Processing locally.\n", req.Data, fromID)
	}
}

// ReallocateResources periodically checks nodes for overload and attempts dynamic reallocation.
func (nm *NodeManager) ReallocateResources() {
	ticker := time.NewTicker(1 * time.Second)
	for range ticker.C {
		nm.Mutex.Lock()
		for _, node := range nm.Nodes {
			node.Mutex.Lock()
			if node.Load > 3 {
				// Create a dummy request to simulate reallocation.
				dummyReq := Request{
					Data:   fmt.Sprintf("ReallocatedTask from Node %d", node.ID),
					Region: node.Region,
					From:   node.ID,
				}
				fmt.Printf("NodeManager: Dynamic reallocation triggered on Node %d (Load: %d)\n", node.ID, node.Load)
				// Offload one unit of work.
				go nm.OffloadRequest(dummyReq, node.ID)
				// Adjust load.
				if node.Load > 0 {
					node.Load--
				}
			}
			node.Mutex.Unlock()
		}
		nm.Mutex.Unlock()
	}
}

func main() {
	rand.Seed(time.Now().UnixNano())
	// Initialize central server with some data.
	centralServer := &CentralServer{
		Data: map[string]string{
			"file1": "Data for file 1",
			"file2": "Data for file 2",
			"file3": "Data for file 3",
		},
		ReplicationFactor: 2,
		ReplicationMap:    make(map[string][]int),
	}

	manager := &NodeManager{
		CentralServer: centralServer,
	}

	var nodes []*Node
	regions := []string{"North", "South"}
	// Create a few nodes.
	for i := 1; i <= 4; i++ {
		region := regions[i%len(regions)]
		node := &Node{
			ID:      i,
			Region:  region,
			Load:    0,
			Data:    make(map[string]string),
			Channel: make(chan Request, 10),
			Manager: manager,
		}
		nodes = append(nodes, node)
		manager.RegisterNode(node)
	}

	var wg sync.WaitGroup
	// Start each node's event loop.
	for _, n := range nodes {
		wg.Add(1)
		go n.Start(&wg)
	}

	// Start dynamic resource reallocation.
	go manager.ReallocateResources()

	// Simulate external requests being sent to random nodes.
	for i := 1; i <= 10; i++ {
		req := Request{
			Data:   fmt.Sprintf("Request %d", i),
			Region: regions[rand.Intn(len(regions))],
			From:   0,
		}
		target := nodes[rand.Intn(len(nodes))]
		fmt.Printf("Main: Sending '%s' (Region: %s) to Node %d\n", req.Data, req.Region, target.ID)
		target.Channel <- req
		time.Sleep(100 * time.Millisecond)
	}

	// Allow time for processing.
	time.Sleep(5 * time.Second)

	// Shut down all nodes.
	for _, n := range nodes {
		close(n.Channel)
	}
	wg.Wait()
	fmt.Println("Simulation complete.")
}
