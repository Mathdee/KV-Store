"use client";
import { useEffect, useState } from "react";
import { getClusterStatus, pauseNode, resumeNode, NodeStatus } from "@/lib/api"; // import pause and resume
import NodeCard from "@/components/NodeCard";
import Console from "@/components/Console";
import BenchmarkPanel from "@/components/BenchmarkPanel";


export default function Dashboard() {
  const [nodes, setNodes] = useState<NodeStatus[]>([]); // stores all node statuses

  // Poll cluster status every 1 second
  useEffect(() => {
    const poll = async () => {
      const status = await getClusterStatus(); // fetch status from all nodes
      setNodes(status); // update state with new data
    };
    poll(); // initial fetch on mount
    const interval = setInterval(poll, 1000); // poll every 1000 milliseconds
    return () => clearInterval(interval); // cleanup interval on unmount
  }, []);

  const handleKill = async (port: number) => { // called when Kill clicked
    await pauseNode(port); // pause the node via API
  };

  const handleRevive = async (port: number) => { // called when Revive clicked
    await resumeNode(port); // resume the node via API
  };

  return (
    <main className="min-h-screen bg-zinc-950 text-white p-8">
      {/* Header */}
      <div className="max-w-5xl mx-auto">
        <h1 className="text-4xl font-bold mb-2">Distributed KV Store</h1>
        <p className="text-zinc-500 mb-8">Raft Consensus • Real-time Dashboard</p>

        {/* Cluster Status */}
        <h2 className="text-xl font-bold mb-4 text-zinc-400">Cluster Status</h2>
        <div className="grid grid-cols-3 gap-4 mb-8">
          {nodes.map((node) => (
            <NodeCard 
              key={node.id} 
              node={node} 
              onKill={() => handleKill(node.port)}   // pass kill handler to card
              onRevive={() => handleRevive(node.port)} // pass revive handler to card
            />
          ))}
        </div>

        {/* Benchmark Panel */}
        <BenchmarkPanel />

        {/* Console */}
        <Console />

        {/* Footer */}
        <p className="text-center text-zinc-600 mt-8 font-mono text-sm">
          Built by Mathijs Deelen,
          Go + Next.js + TailwindCSS + Raft Consensus Algorithm
        </p>
      </div>
    </main>
  );
}