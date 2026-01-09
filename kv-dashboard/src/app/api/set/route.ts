import { NextRequest, NextResponse } from "next/server";
import net from "net";

const NODES = [8080, 8081, 8082];

/**
 * Send a TCP command to a KV store node
 */
async function sendCommand(port: number, command: string, timeoutMs: number = 5000): Promise<string> {
  return new Promise((resolve, reject) => {
    const client = new net.Socket();
    client.setTimeout(timeoutMs);

    client.connect(port, "localhost", () => {
      client.write(command + "\n");
    });

    client.on("data", (data) => {
      client.destroy();
      resolve(data.toString().trim());
    });

    client.on("error", (err) => {
      client.destroy();
      reject(err);
    });

    client.on("timeout", () => {
      client.destroy();
      reject(new Error("timeout"));
    });
  });
}

/**
 * Query all nodes to find the current Raft leader
 */
async function findLeader(): Promise<number | null> {
  for (const port of NODES) {
    try {
      const res = await fetch(`http://localhost:${port + 1000}/status`);
      const data = await res.json();
      if (data.state === "Leader") {
        return port;
      }
    } catch {}
  }
  return null;
}

/**
 * Jittered exponential backoff - prevents thundering herd problem
 * 
 * Base delay doubles each attempt: 50ms, 100ms, 200ms, 400ms...
 * Jitter adds randomness (0-100% of delay) so retries don't all hit at once
 * 
 * Example: attempt 3, base 50ms
 *   - Exponential: 50 * 2^2 = 200ms
 *   - Jitter: random 0-200ms
 *   - Total: 200-400ms
 */
function sleepWithJitter(baseMs: number, attempt: number, maxMs: number): Promise<void> {
  const exponentialDelay = baseMs * Math.pow(2, attempt - 1);
  const cappedDelay = Math.min(exponentialDelay, maxMs);
  const jitter = Math.random() * cappedDelay;  // 0% to 100% of delay
  const finalDelay = cappedDelay + jitter;
  
  return new Promise(resolve => setTimeout(resolve, finalDelay));
}

/**
 * SET endpoint with automatic leader discovery and retry
 * 
 * Features:
 * - Finds current Raft leader automatically
 * - Retries with exponential backoff + jitter on failure
 * - Handles leader elections transparently
 */
export async function GET(request: NextRequest) {
  const { searchParams } = new URL(request.url);
  let port = parseInt(searchParams.get("port") || "8080");
  const key = searchParams.get("key") || "";
  const value = searchParams.get("value") || "";

  const command = `SET ${key} ${value}`;
  
  // Retry configuration
  const maxAttempts = 7;       // Total attempts before giving up
  const baseDelayMs = 50;      // Starting delay for backoff
  const maxDelayMs = 2000;     // Maximum delay cap
  
  let attempt = 0;

  while (attempt < maxAttempts) {
    try {
      const result = await sendCommand(port, command);

      // Success - return immediately
      if (result === "OK") {
        return NextResponse.json({ 
          success: true, 
          result, 
          attempts: attempt + 1 
        });
      }

      // Not the leader - find new leader for next attempt
      if (result === "NOTLEADER" || result.includes("not leader")) {
        const leaderPort = await findLeader();
        if (leaderPort) {
          port = leaderPort;
        }
        // Don't count NOTLEADER as a failure - it's a redirect
      }
    } catch (err) {
      // Connection error - try to find leader
      const leaderPort = await findLeader();
      if (leaderPort) {
        port = leaderPort;
      }
    }

    attempt++;
    
    // Wait before retry (with jitter to prevent thundering herd)
    if (attempt < maxAttempts) {
      await sleepWithJitter(baseDelayMs, attempt, maxDelayMs);
    }
  }

  // All retries exhausted
  return NextResponse.json({ 
    success: false, 
    result: "Max retries exceeded",
    attempts: maxAttempts 
  });
}