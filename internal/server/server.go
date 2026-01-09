package server

import (
	"bufio"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/mathdee/KV-Store/internal/raft"

	"github.com/mathdee/KV-Store/internal/store"
)

type Server struct {
	store   *store.Store
	peers   []string // creates a slice of strings to store the addresses of the replicas.
	raft    *raft.Consensus
	metrics *Metrics
}

func NewServer(s *store.Store, r *raft.Consensus) *Server {
	return &Server{store: s, raft: r, metrics: NewMetrics()}
}

func parseInt(s string) int {
	n, _ := strconv.Atoi(s) //converts string to int
	return n
}

// Start opens the socket and listens for connections
func (s *Server) Start(port string) error {
	//net.Listen creates a socket bound to a port (e,g., 8080)

	ln, err := net.Listen("tcp", port)

	if err != nil {
		return err
	}
	defer ln.Close()

	fmt.Printf("Server listening on port %s -->  \n", port)

	for {
		// Accept() blocks until a client connects
		// It returns a 'conn' object representing the connection to THAT sepcific client.
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println("Connection error: ", err)
			continue
		}

		// KEY CONCEPT: Goroutines
		// We launch a background thread for this specific client.
		// This allows the main loop to immediately go back to waiting for the NEXt client.

		go s.handleConnection(conn)
	}
}

func (s *Server) Join(peerAddress string) { //method that adds a peer to the server
	s.peers = append(s.peers, peerAddress)      // adds peer address to the slice
	fmt.Printf("Added peer: %s\n", peerAddress) // prints the peer address
}

func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close() // Makes sure connection closes when function finishes

	//REad from the connection like a file
	scanner := bufio.NewScanner(conn)

	//Loop over every line sent by the client
	for scanner.Scan() {
		text := scanner.Text()
		parts := strings.Fields(text) // SPlit by whitespace

		if len(parts) == 0 {
			continue
		}

		cmd := parts[0]
		//Start timing for GET and SET commands
		var opStart time.Time
		shouldRecord := cmd == "SET" || cmd == "GET"
		if shouldRecord {
			opStart = time.Now()
		}
		switch cmd {
		case "SET":
			if len(parts) < 3 {
				fmt.Fprintln(conn, "ERR Usage: SET key value")
				return
			}
			key := parts[1]
			value := strings.Join(parts[2:], " ")
			// Check if the server is the leader.
			isLeader := s.raft.GetState() == "Leader"
			if isLeader {
				s.raft.Replicate("SET " + key + " " + value)
				s.store.Set(key, value)
				fmt.Fprintln(conn, "OK")
				if shouldRecord {
					s.metrics.RecordSuccess(time.Since(opStart))
				}
			} else {
				// Tell client who the leader is so they can retry
				// Format: "NOTLEADER <leader_port>"
				// We don't track leader, so client must discover
				fmt.Fprintln(conn, "NOTLEADER")

			}

		case "APPENDENTRIES":
			if len(parts) < 5 {
				continue
			}

			term := parseInt(parts[1])
			leaderID := parts[2]
			prevLogIndex := parseInt(parts[3]) // NEW: where to start appending
			entryCount := parseInt(parts[4])

			// Read the incoming entries
			var newEntries []raft.LogEntry
			for i := 0; i < entryCount; i++ {
				if scanner.Scan() {
					line := scanner.Text()
					commaIdx := strings.Index(line, ",")
					if commaIdx == -1 {
						continue
					}
					entryTerm := parseInt(line[:commaIdx])
					entryCmd := line[commaIdx+1:]
					newEntries = append(newEntries, raft.LogEntry{
						Term:    entryTerm,
						Command: entryCmd,
					})
				}
			}

			// Call updated handler and get result
			success := s.raft.HandleAppendEntriesIncremental(term, leaderID, prevLogIndex, newEntries)

			if success {
				fmt.Fprintln(conn, "SUCCESS")

				// Apply new entries to store
				unapplied := s.raft.GetUnappliedEntries()
				for _, entry := range unapplied {
					cmdParts := strings.Fields(entry.Command)
					if len(cmdParts) >= 3 && cmdParts[0] == "SET" {
						val := strings.Join(cmdParts[2:], " ")
						s.store.Set(cmdParts[1], val)
					}
				}
			} else {
				fmt.Fprintln(conn, "CONFLICT")
			}
		case "GET":
			if len(parts) < 2 {
				fmt.Fprint(conn, "ERR usage: GET key")
				continue
			}
			val, err := s.store.Get(parts[1])

			if err != nil {
				fmt.Fprintln(conn, "(nil)")
			} else {
				fmt.Fprintln(conn, val)
			}
			if shouldRecord {
				s.metrics.RecordSuccess(time.Since(opStart))
			}

		case "JOIN": // Handles JOIN command from client
			if len(parts) != 2 { // Checks for address argument
				fmt.Fprintln(conn, "ERR usage: JOIN address") // Prints usage error if missing
				continue                                      // Skips rest, waits next input
			}
			s.Join(parts[1])         // Adds peer address to server
			fmt.Fprintln(conn, "OK") // Acknowledges successful join

		case "VOTEREQUEST":
			if len(parts) < 3 {
				continue
			}
			term := parseInt(parts[1])
			candidateID := parts[2]

			granted := s.raft.HandleRequestVote(term, candidateID)
			if granted {
				fmt.Fprint(conn, "VOTEGRANTED\n")
			} else {
				fmt.Fprint(conn, "VOTEDENIED\n")
			}

		case "HEARTBEAT":
			if len(parts) < 2 {
				continue
			}
			term := parseInt(parts[1])
			s.raft.HandleHeartbeat(term)

		default: // Handles unknown commands from client
			fmt.Fprintln(conn, "ERR unknown command") // Prints error for unknown command

		}
	}

}

func (s *Server) GetMetrics() *Metrics {
	return s.metrics
}

// net.Listen creates a new TCP socket that listens for incoming connections.
// net.Dial creates a new TCP connection to the peer server.
// replicate() the server becomes a client temporarily to send the SET command to the peer servers.
