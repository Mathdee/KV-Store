import { NodeStatus } from "@/lib/api";

interface Props {
  node: NodeStatus;
  onKill: () => void;   // called when Kill button clicked
  onRevive: () => void; // called when Revive button clicked
}

export default function NodeCard({ node, onKill, onRevive }: Props) {
  const stateColors = {
    Leader: "bg-emerald-500 shadow-emerald-500/50",    // green for leader node
    Follower: "bg-blue-500 shadow-blue-500/50",       // blue for follower node
    Candidate: "bg-amber-500 shadow-amber-500/50 animate-pulse", // amber pulsing for candidate
    Dead: "bg-red-500/50",                            // red for dead or paused
  };

  return (
    <div className={`p-6 rounded-xl border border-zinc-800 bg-zinc-900 ${node.alive ? "" : "opacity-50"}`}>
      {/* Status indicator */}
      <div className="flex items-center gap-3 mb-4">
        <div className={`w-4 h-4 rounded-full ${stateColors[node.state]} shadow-lg`} />
        <span className="text-zinc-400 font-mono">:{node.port}</span>
      </div>

      {/* State */}
      <h3 className="text-2xl font-bold text-white mb-2">
        {node.paused ? "Paused" : node.state}
      </h3>

      {/* Stats */}
      <div className="space-y-1 text-sm text-zinc-500 font-mono">
        <p>Term: {node.term}</p>
        <p>Log: {node.logLength} entries</p>
      </div>

      {/* Kill or Revive button based on state */}
      {node.paused ? (
        <button
          onClick={onRevive}  // call revive handler when clicked
          className="mt-4 px-4 py-2 bg-emerald-500/20 text-emerald-400 rounded-lg hover:bg-emerald-500/30 transition font-mono text-sm"
        >
          Revive Node
        </button>
      ) : node.alive ? (
        <button
          onClick={onKill}  // call kill handler when clicked
          className="mt-4 px-4 py-2 bg-red-500/20 text-red-400 rounded-lg hover:bg-red-500/30 transition font-mono text-sm"
        >
          Kill Node
        </button>
      ) : null}
    </div>
  );
}