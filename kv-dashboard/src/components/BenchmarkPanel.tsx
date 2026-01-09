"use client";
import { useState } from "react";

interface MetricsData {
  totalRequests: number;
  successCount: number;
  failCount: number;
  throughput: number;
  latencyAvgMs: number;
  latencyP50Ms: number;
  latencyP95Ms: number;
  latencyP99Ms: number;
  uptimeSeconds: number;
}

const NODES = [
  { tcp: 8080, http: 9080 },
  { tcp: 8081, http: 9081 },
  { tcp: 8082, http: 9082 },
];

export default function BenchmarkPanel() {
  const [running, setRunning] = useState(false);
  const [metrics, setMetrics] = useState<MetricsData | null>(null);
  const [progress, setProgress] = useState(0);
  const [leaderPort, setLeaderPort] = useState<number | null>(null);
  const [totalOps, setTotalOps] = useState(5000);
  const [batchSize, setBatchSize] = useState(100);
  const [clearing, setClearing] = useState(false);

  const findLeader = async (): Promise<{ tcp: number; http: number } | null> => {
    for (const node of NODES) {
      try {
        const res = await fetch(`http://localhost:${node.http}/status`);
        const data = await res.json();
        if (data.state === "Leader") {
          return node;
        }
      } catch {}
    }
    return null;
  };

  const runBenchmark = async () => {
    setRunning(true);
    setProgress(0);
    setMetrics(null);
  
    const opsPerBatch = batchSize; // Use state variable, not hardcoded!
    const totalBatches = Math.ceil(totalOps / opsPerBatch);
    
    // Aggregate metrics across batches
    let totalSuccess = 0;
    let totalFailed = 0;
    
    // Weighted latency tracking
    let weightedLatencySum = 0;
    let weightedP50Sum = 0;
    let weightedP95Sum = 0;
    let weightedP99Sum = 0;
    
    const startTime = Date.now();
  
    for (let batch = 0; batch < totalBatches; batch++) {
      // Find current leader (may change between batches!)
      const leader = await findLeader();
      if (!leader) {
        console.log("No leader available, waiting...");
        await new Promise(r => setTimeout(r, 500));
        batch--; // Retry this batch
        continue;
      }
      setLeaderPort(leader.tcp);
  
      const opsThisBatch = Math.min(opsPerBatch, totalOps - (batch * opsPerBatch));
  
      try {
        const res = await fetch(
          `http://localhost:${leader.http}/benchmark?requests=${opsThisBatch}&concurrency=100`,
          { signal: AbortSignal.timeout(30000) }
        );
        const data = await res.json();
  
        if (data.successful > 0) {
          totalSuccess += data.successful;
          totalFailed += data.failed || 0;
          
          // Weight latencies by batch size for proper averaging
          const weight = data.successful;
          weightedLatencySum += (data.latencyAvgMs || 0) * weight;
          weightedP50Sum += (data.latencyP50Ms || 0) * weight;
          weightedP95Sum += (data.latencyP95Ms || 0) * weight;
          weightedP99Sum += (data.latencyP99Ms || 0) * weight;
        }
      } catch (err) {
        console.log(`Batch ${batch} failed, leader may have changed. Retrying...`);
        batch--;
        await new Promise(r => setTimeout(r, 200));
        continue;
      }
  
      setProgress(Math.round(((batch + 1) / totalBatches) * 100));
    }
  
    const totalDuration = (Date.now() - startTime) / 1000;
  
    // Calculate weighted average latencies
    setMetrics({
      totalRequests: totalOps,
      successCount: totalSuccess,
      failCount: totalFailed,
      throughput: totalSuccess / totalDuration,
      latencyAvgMs: totalSuccess > 0 ? weightedLatencySum / totalSuccess : 0,
      latencyP50Ms: totalSuccess > 0 ? weightedP50Sum / totalSuccess : 0,
      latencyP95Ms: totalSuccess > 0 ? weightedP95Sum / totalSuccess : 0,
      latencyP99Ms: totalSuccess > 0 ? weightedP99Sum / totalSuccess : 0,
      uptimeSeconds: totalDuration,
    });
  
    setProgress(100);
    setRunning(false);
  };

  const clearData = async () => {
    setClearing(true);
    for (const node of NODES) {
      try {
        await fetch(`http://localhost:${node.http}/clear`);
      } catch {}
    }
    setMetrics(null);
    setClearing(false);
  };

  return (
    <div className="bg-zinc-900 border border-zinc-800 rounded-xl p-6 mb-8">
      <h2 className="text-xl font-bold mb-4 text-zinc-400">⚡ Performance Benchmark</h2>

      {/* Configuration */}
      <div className="flex flex-wrap gap-4 mb-6">
        <div className="flex flex-col">
          <label className="text-xs text-zinc-500 font-mono mb-1">TOTAL REQUESTS</label>
          <input
            type="number"
            value={totalOps}
            onChange={(e) => setTotalOps(Math.max(1, parseInt(e.target.value) || 1))}
            disabled={running}
            className="w-32 px-3 py-2 bg-zinc-800 border border-zinc-700 rounded-lg text-white font-mono focus:border-emerald-500 focus:outline-none"
          />
        </div>
        <div className="flex flex-col">
          <label className="text-xs text-zinc-500 font-mono mb-1">BATCH SIZE</label>
          <input
            type="number"
            value={batchSize}
            onChange={(e) => setBatchSize(Math.max(1, parseInt(e.target.value) || 1))}
            disabled={running}
            className="w-32 px-3 py-2 bg-zinc-800 border border-zinc-700 rounded-lg text-white font-mono focus:border-emerald-500 focus:outline-none"
          />
        </div>
      </div>

      {leaderPort && (
        <div className="text-sm text-emerald-400 mb-4 font-mono">
          📍 Leader: node :{leaderPort}
        </div>
      )}

      {/* Buttons */}
      <div className="flex gap-3 mb-6">
        <button
          onClick={runBenchmark}
          disabled={running || clearing}
          className={`px-6 py-3 rounded-lg font-mono font-bold transition ${
            running || clearing
              ? "bg-zinc-700 text-zinc-500 cursor-not-allowed"
              : "bg-gradient-to-r from-emerald-500 to-cyan-500 text-black hover:from-emerald-400 hover:to-cyan-400"
          }`}
        >
          {running ? `Running... ${progress}%` : `🚀 Run ${totalOps.toLocaleString()} Requests`}
        </button>

        <button
          onClick={clearData}
          disabled={running || clearing}
          className={`px-6 py-3 rounded-lg font-mono font-bold transition ${
            running || clearing
              ? "bg-zinc-700 text-zinc-500 cursor-not-allowed"
              : "bg-red-500/20 text-red-400 border border-red-500/30 hover:bg-red-500/30"
          }`}
        >
          {clearing ? "Clearing..." : "🗑️ Clear Data"}
        </button>
      </div>

      {/* Progress */}
      {running && (
        <div className="w-full bg-zinc-800 rounded-full h-2 mb-6">
          <div
            className="bg-gradient-to-r from-emerald-500 to-cyan-500 h-2 rounded-full transition-all duration-300"
            style={{ width: `${progress}%` }}
          />
        </div>
      )}

      {/* Results */}
      {metrics && (
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
          <MetricBox label="Throughput" value={`${metrics.throughput.toFixed(0)} ops/sec`} color="emerald" />
          <MetricBox label="Total Requests" value={metrics.totalRequests.toLocaleString()} color="blue" />
          <MetricBox label="Success Rate" value={`${((metrics.successCount / metrics.totalRequests) * 100).toFixed(1)}%`} color="cyan" />
          <MetricBox label="Duration" value={`${metrics.uptimeSeconds.toFixed(2)}s`} color="amber" />
          <MetricBox label="Latency (avg)" value={`${metrics.latencyAvgMs.toFixed(2)} ms`} color="purple" />
          <MetricBox label="Latency (p50)" value={`${metrics.latencyP50Ms.toFixed(2)} ms`} color="purple" />
          <MetricBox label="Latency (p95)" value={`${metrics.latencyP95Ms.toFixed(2)} ms`} color="purple" />
          <MetricBox label="Latency (p99)" value={`${metrics.latencyP99Ms.toFixed(2)} ms`} color="red" />
        </div>
      )}
    </div>
  );
}

function MetricBox({ label, value, color }: { label: string; value: string; color: string }) {
  const colors: Record<string, string> = {
    emerald: "border-emerald-500/30 text-emerald-400",
    blue: "border-blue-500/30 text-blue-400",
    cyan: "border-cyan-500/30 text-cyan-400",
    amber: "border-amber-500/30 text-amber-400",
    purple: "border-purple-500/30 text-purple-400",
    red: "border-red-500/30 text-red-400",
  };

  return (
    <div className={`bg-zinc-800/50 border ${colors[color]} rounded-lg p-4`}>
      <div className="text-zinc-500 text-xs font-mono uppercase mb-1">{label}</div>
      <div className={`text-xl font-bold font-mono ${colors[color].split(" ")[1]}`}>{value}</div>
    </div>
  );
}