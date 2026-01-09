const NODES = [
    { id: "node1", tcp: 8080, http: 9080 },
    { id: "node2", tcp: 8081, http: 9081 },
    { id: "node3", tcp: 8082, http: 9082 },
  ];
  
  export interface NodeStatus {
    id: string;
    port: number;
    alive: boolean;
    state: "Leader" | "Follower" | "Candidate" | "Dead"; // possible node states
    term: number;
    logLength: number;
    paused: boolean; // true when node is paused
  }
  
  // Fetch status from all nodes
  export async function getClusterStatus(): Promise<NodeStatus[]> {
    const results = await Promise.all(
      NODES.map(async (node) => {
        try {
          const res = await fetch(`http://localhost:${node.http}/status`, {
            cache: "no-store", // always fetch fresh data
          });
          const data = await res.json(); // parse JSON response from server
          return {
            id: node.id,
            port: node.tcp,
            alive: !data.paused, // node is alive if not paused
            state: data.paused ? "Dead" as const : data.state, // show Dead if paused
            term: data.term,
            logLength: data.logLength,
            paused: data.paused || false, // include paused state from server
          };
        } catch {
          return {
            id: node.id,
            port: node.tcp,
            alive: false, // unreachable nodes are not alive
            state: "Dead" as const, // show as Dead in UI
            term: 0,
            logLength: 0,
            paused: false, // not paused, just unreachable
          };
        }
      })
    );
    return results;
  }
  
  // Pause a node - simulates failure
  export async function pauseNode(port: number): Promise<void> {
    const httpPort = port + 1000; // HTTP port is TCP + 1000
    try {
      await fetch(`http://localhost:${httpPort}/pause`); // call pause endpoint on server
    } catch {
      // Request might fail, that's okay
    }
  }

  // Resume a paused node - rejoins cluster
  export async function resumeNode(port: number): Promise<void> {
    const httpPort = port + 1000; // HTTP port is TCP + 1000
    try {
      await fetch(`http://localhost:${httpPort}/resume`); // call resume endpoint on server
    } catch {
      // Request might fail, that's okay
    }
  }
  
  // SET key value (tries each node until leader found)
  export async function setKey(key: string, value: string): Promise<{ success: boolean; leader?: string }> {
    for (const node of NODES) {
      try {
        // Using a simple proxy approach - you'll need a small API route for TCP
        const res = await fetch(`/api/set?port=${node.tcp}&key=${key}&value=${value}`);
        const data = await res.json();
        if (data.success) {
          return { success: true, leader: node.id };
        }
      } catch {
        continue;
      }
    }
    return { success: false };
  }
  
  // GET key
  export async function getKey(key: string): Promise<{ value: string | null; from?: string }> {
    for (const node of NODES) {
      try {
        const res = await fetch(`/api/get?port=${node.tcp}&key=${key}`);
        const data = await res.json();
        if (data.value) {
          return { value: data.value, from: node.id };
        }
      } catch {
        continue;
      }
    }
    return { value: null };
  }