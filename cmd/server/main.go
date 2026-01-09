package main // program entry point

import (
	"flag"
	"fmt" // print messages to screen
	"log" // record errors and events
	"strconv"
	_ "strconv"
	"strings"

	"github.com/mathdee/KV-Store/internal/raft"
	"github.com/mathdee/KV-Store/internal/server" // handles network connections
	"github.com/mathdee/KV-Store/internal/store"  // manages data storage
	"github.com/mathdee/KV-Store/internal/wal"    // backup log for safety
)

func main() { // program starts here

	port := flag.String("port", "8080", "listen on this port") // Define a flag for the port.
	// port := 8080,  "8080" = default port, "listen on this port" = helps to understand the flag.

	replica := flag.String("replica", "", "Primary or secondary server") // Define a flag for the replica
	peersFlag := flag.String("peers", "", "Comma-separated list of peer addresses")
	flag.Parse() // parses the flags and sets their values to the variables.

	id := ":" + *port

	var peers []string
	if *peersFlag != "" {
		peers = strings.Split(*peersFlag, ",")
	}
	var logFile string // variable stores log file name & value in empty string.
	if *replica != "" {
		logFile = fmt.Sprintf("server_%s.log", *replica)
	} else {
		logFile = fmt.Sprintf("server_%s.log", *port)
	}

	// Intialize the Write-Ahead Log
	w, err := wal.NewWAL(logFile) // create backup log file
	if err != nil {               // if something went wrong
		log.Fatalf("Failed to init WAL: %v", err) // show error and stop
	}
	defer w.Close() // close file when done

	// Part that recovers the data from the disk
	fmt.Printf("Recovering data from disk %s\n", logFile) // notify user of recovery
	data, err := wal.Recover(logFile)                     // load saved data from backup
	if err != nil {                                       // if recovery failed
		log.Fatalf("Failed to recover WAL: %v", err) // show error and stop
	}

	// Creates data storage system
	s := store.NewStore(w) // create data storage system
	s.Restore(data)        // restore saved data

	// Starts the server
	consensus := raft.NewConsensus(id, peers)
	consensus.Start()
	tcpPort, _ := strconv.Atoi(*port)
	httpPort := fmt.Sprintf(":%d", tcpPort+1000)
	srv := server.NewServer(s, consensus)                              // Create network server
	httpServer := server.NewHTTPServer(consensus, srv.GetMetrics(), s) // Create HTTP server and pass the store
	go httpServer.Start(httpPort)                                      // Start HTTP server in background

	if *replica != "" {
		fmt.Printf("I am a replica of port %s\n: ", *replica) // prints the port of replica
	} else {
		fmt.Printf("I am the primary server\n") // prints the primary server
	}

	address := ":" + *port                     // creates a string value e.g: ":8080"
	if err := srv.Start(address); err != nil { // starts the server and checks it it fails then logs the error.
		log.Fatal(err)
	}
}
