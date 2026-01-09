package raft

import (
	"fmt"
	"math/rand"
	"net"
	"strings"
	"sync"
	"time"
)

const (
	Follower  = "Follower"
	Candidate = "Candidate"
	Leader    = "Leader"
)

type LogEntry struct {
	Term    int
	Command string // SET, GET, JOIN commands.
}
type Consensus struct {
	mu          sync.Mutex // mutex, allows only one goroutine to access the struct at a time.
	State       string     //current state of server
	CurrentTerm int        // current term number
	ID          string     // ID of curr server
	Peers       []string   // list of all server addresses
	VotedFor    string     // ID of the server the current server voted for
	heartbeatCh chan bool  // channel to send and receive heartbeat messages
	Log         []LogEntry
	CommitIndex int  // index of commited log entries
	lastApplied int  // index of last applied log entry
	paused      bool // stops node from Raft participation

	nextIndex  map[string]int // nextIndex for each peer
	matchIndex map[string]int // matchIndex for each peer
}

func NewConsensus(id string, peers []string) *Consensus { // create Consensus struct for Raft node
	return &Consensus{
		State:       Follower,        // set initial state to Follower
		CurrentTerm: 0,               // term starts at zero, Raft default
		ID:          id,              // set this node's unique ID
		Peers:       peers,           // assign peer server addresses list
		heartbeatCh: make(chan bool), // create channel for heartbeat signals
		Log:         []LogEntry{},    // initialize empty log.
		CommitIndex: -1,              // -1 means no commits yet.
		lastApplied: -1,
		paused:      false,                // node starts active, not paused
		nextIndex:   make(map[string]int), // nextIndex for each peer
		matchIndex:  make(map[string]int), // matchIndex for each peer
	}
}

func (c *Consensus) GetLogLength() int { //Gets the length of log to know nb of entries.
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.Log)
}

// Start of Raft Election Processss

func (c *Consensus) Start() {
	go func() {
		for {
			c.mu.Lock()
			state := c.State
			c.mu.Unlock()

			switch state {
			case Follower:
				c.runFollower()
			case Candidate:
				c.runCandidate()
			case Leader:
				c.runLeader()
			default:
				fmt.Println("Unknown state")
			}
		}
	}()
}

func (c *Consensus) GetUnappliedEntries() []LogEntry {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.lastApplied >= len(c.Log)-1 { // if last applies >=  then length of log entries -1, return nill.
		return nil
	}

	start := c.lastApplied + 1
	entries := c.Log[start:]
	c.lastApplied = len(c.Log) - 1
	return entries
}

// Follower logic, runFollower() method
func (c *Consensus) runFollower() {
	if c.IsPaused() { // check if node is paused
		time.Sleep(100 * time.Millisecond) // sleep to avoid busy spinning
		return                             // exit early, skip Raft logic
	}

	timeout := time.Duration(500+rand.Intn(500)) * time.Millisecond // 500-1000ms timeout
	timer := time.NewTimer(timeout)

	select {
	case <-c.heartbeatCh:
		timer.Stop()
		return
	case <-timer.C:
		fmt.Printf("[%s] Timeout! Starting Election -> \n", c.ID)
		c.mu.Lock()
		c.State = Candidate
		c.mu.Unlock()
	}
}

// Candidate logic, runCandidate() method

func (c *Consensus) runCandidate() {
	if c.IsPaused() {
		time.Sleep(100 * time.Millisecond)
		return
	}

	c.mu.Lock()
	c.CurrentTerm++
	c.VotedFor = c.ID
	votes := 1
	term := c.CurrentTerm
	c.mu.Unlock()

	fmt.Printf("[%s] Candidate Election term %d\n", c.ID, term)

	voteCh := make(chan bool, len(c.Peers))
	for _, peer := range c.Peers {
		go c.requestVoteFromPeer(peer, term, voteCh)
	}

	timeout := time.After(500 * time.Millisecond) // Timeout BEFORE the loop

	for {
		select {
		case granted := <-voteCh:
			if granted {
				votes++
			}
			quorum := (len(c.Peers)+1)/2 + 1

			if votes >= quorum {
				fmt.Printf("[%s] Won the Election! with %d votes\n", c.ID, votes)
				c.mu.Lock()
				c.State = Leader

				// Initialize nextIndex for all peers
				for _, peer := range c.Peers {
					c.nextIndex[peer] = len(c.Log)
					c.matchIndex[peer] = -1 // -1 means no entries matched yet
				}

				c.mu.Unlock()
				return
			}

		case <-timeout:
			fmt.Printf("[%s] Election failed! Timeout, back to Follower.\n", c.ID)
			c.mu.Lock()
			c.State = Follower
			c.mu.Unlock()
			return
		}
	}
}

// Leader logic, runLeader() method

func (c *Consensus) runLeader() {
	if c.IsPaused() { // check if node is paused
		time.Sleep(100 * time.Millisecond) // sleep to avoid busy spinning
		return                             // exit early, skip Raft logic
	}

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			go c.broadcastHeartbeat()
		}
		c.mu.Lock()

		if c.State != "Leader" || c.paused {
			c.mu.Unlock()
			return
		}
		c.mu.Unlock()

	}

}

// Request Vote from Peer, requestVoteFromPeer() method.

func (c *Consensus) requestVoteFromPeer(peer string, term int, voteCh chan bool) {
	conn, err := net.Dial("tcp", peer)
	if err != nil {
		voteCh <- false
		return
	}

	defer conn.Close()

	fmt.Fprintf(conn, "VOTEREQUEST %d %s\n", term, c.ID)

	// implementing the request to the peer.
	buf := make([]byte, 1024) // stores the response from the peer.
	n, _ := conn.Read(buf)
	response := strings.TrimSpace(string(buf[:n])) // converts response to string so we can parse it.

	if response == "VOTEGRANTED" {
		voteCh <- true
	} else {
		voteCh <- false
	}
}

func (c *Consensus) broadcastHeartbeat() {
	c.mu.Lock()
	term := c.CurrentTerm
	leaderID := c.ID
	logLen := len(c.Log)
	c.mu.Unlock()

	for _, peer := range c.Peers {
		go func(p string) {
			c.mu.Lock()

			if _, exists := c.nextIndex[p]; !exists {
				c.nextIndex[p] = logLen // set nextIndex to log length for new peers
				c.matchIndex[p] = 0     // set matchIndex to 0 for new peers
			}

			nextIdx := c.nextIndex[p]

			// Determine what entries to send
			var entriesToSend []LogEntry
			if nextIdx < logLen {
				// Follower is behind - send only missing entries
				entriesToSend = c.Log[nextIdx:]
			}
			// else: follower is up-to-date, send empty (pure heartbeat)

			c.mu.Unlock()

			conn, err := net.Dial("tcp", p)
			if err != nil {
				return
			}
			defer conn.Close()

			// Protocol: APPENDENTRIES <Term> <LeaderID> <PrevLogIndex> <EntryCount>
			prevLogIndex := nextIdx - 1
			fmt.Fprintf(conn, "APPENDENTRIES %d %s %d %d\n", term, leaderID, prevLogIndex, len(entriesToSend))

			// Send only the NEW entries (not the full log!)
			for _, entry := range entriesToSend {
				fmt.Fprintf(conn, "%d,%s\n", entry.Term, entry.Command)
			}

			// Read response
			buf := make([]byte, 64)
			n, err := conn.Read(buf)
			if err != nil {
				return
			}
			response := strings.TrimSpace(string(buf[:n]))

			c.mu.Lock()
			defer c.mu.Unlock()

			if response == "SUCCESS" {
				// Follower accepted - update tracking
				c.nextIndex[p] = logLen
				c.matchIndex[p] = logLen - 1
			} else if response == "CONFLICT" {
				// Log mismatch - back up and retry next time
				if c.nextIndex[p] > 0 {
					c.nextIndex[p]--
				}
			}
		}(peer)
	}
}

func (c *Consensus) Replicate(command string) bool {
	c.mu.Lock()
	if c.State != Leader {
		c.mu.Unlock()
		return false //Only leader can replicate data.
	}
	entry := LogEntry{Term: c.CurrentTerm, Command: command}
	c.Log = append(c.Log, entry)
	c.mu.Unlock()

	fmt.Printf("[%s] Leader queued entry: %s\n", c.ID, command)
	c.broadcastHeartbeat() // sends heartbeat to all followers to replicate the data.
	return true

}

// handle requestvote from peer, handleRequestVoteFromPeer() method.
// (Reads request from peer and sends response.)

func (c *Consensus) HandleRequestVote(term int, candidateID string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if term < c.CurrentTerm { // if the term is older than current -> reject.
		return false
	}

	if term > c.CurrentTerm { // if the term is newer than current -> update current term and become follower.
		c.CurrentTerm = term
		c.State = Follower
		c.VotedFor = ""
	}

	if c.VotedFor == "" || c.VotedFor == candidateID { // if not voted for anyone or voted for the candidate -> grant vote.
		c.VotedFor = candidateID

		// this go func() is used to reset the heartbeat timer because we're a follower now.
		go func() {
			c.heartbeatCh <- true
		}()
		return true
	}
	return false
}

func (c *Consensus) HandleHeartbeat(term int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if term >= c.CurrentTerm {
		c.CurrentTerm = term
		c.State = Follower
		// this go func() is used to reset the heartbeat timer because we're a follower now.
		go func() {
			c.heartbeatCh <- true
		}()
	}
}

func (c *Consensus) GetState() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.State
}
func (c *Consensus) GetTerm() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.CurrentTerm
}
func (c *Consensus) GetCommitIndex() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.CommitIndex
}

func (c *Consensus) Pause() { // stops node from cluster participation
	c.mu.Lock()         // lock mutex for thread-safe access
	defer c.mu.Unlock() // unlock when function returns safely
	c.paused = true     // set paused flag to true
	fmt.Printf("[%s] Node PAUSED - simulating failure\n", c.ID)
}

func (c *Consensus) Resume() { // restarts node to rejoin cluster
	c.mu.Lock()         // lock mutex for thread-safe access
	defer c.mu.Unlock() // unlock when function returns safely
	c.paused = false    // set paused flag to false
	c.State = Follower  // rejoin cluster as a follower
	c.VotedFor = ""     // reset vote for new elections
	fmt.Printf("[%s] Node RESUMED - rejoining cluster\n", c.ID)
}

func (c *Consensus) IsPaused() bool { // checks if node is paused
	c.mu.Lock()         // lock mutex for thread-safe access
	defer c.mu.Unlock() // unlock when function returns safely
	return c.paused     // return current paused state value
}

// ClearLog removes all benchmark entries from the log
func (c *Consensus) ClearLog() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Log = []LogEntry{}
	c.CommitIndex = 0
	c.lastApplied = 0
	fmt.Printf("[%s] Log cleared\n", c.ID)
}

// AddLogEntry adds to log without triggering heartbeat (for benchmarks)
func (c *Consensus) AddLogEntry(command string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.paused {
		return // don't add entries if node is paused
	}
	entry := LogEntry{Term: c.CurrentTerm, Command: command}
	c.Log = append(c.Log, entry)
}

// HandleAppendEntriesIncremental handles incremental log replication (proper Raft)
func (c *Consensus) HandleAppendEntriesIncremental(term int, leaderID string, prevLogIndex int, entries []LogEntry) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.paused {
		return false // don't process entries if node is paused
	}
	// Reject if term is old
	if term < c.CurrentTerm {
		return false
	}

	// Update term and become follower
	c.CurrentTerm = term
	c.State = Follower
	c.VotedFor = ""

	// Reset election timer
	go func() { c.heartbeatCh <- true }()

	// If this is a pure heartbeat (no entries), just accept
	if len(entries) == 0 {
		return true
	}

	// Log matching: check if we have the entry at prevLogIndex
	// (Simplified: we trust leader for now, proper impl would check term match)

	// Append new entries starting at prevLogIndex + 1
	insertPoint := prevLogIndex + 1

	if insertPoint < 0 {
		insertPoint = 0
	}

	// Truncate conflicting entries and append new ones
	if insertPoint <= len(c.Log) {
		c.Log = c.Log[:insertPoint]
	}
	c.Log = append(c.Log, entries...)

	return true
}
