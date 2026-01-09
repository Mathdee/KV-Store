"use client";
import { useState } from "react";
import { setKey, getKey } from "@/lib/api";

export default function Console() {
  const [key, setKeyInput] = useState("");
  const [value, setValue] = useState("");
  const [output, setOutput] = useState<string[]>([]);

  const handleSet = async () => {
    if (!key || !value) return;
    const result = await setKey(key, value);
    setOutput((prev) => [
      ...prev,
      `> SET ${key} ${value}`,
      result.success ? `OK (via ${result.leader})` : "ERR: No leader available",
    ]);
  };

  const handleGet = async () => {
    if (!key) return;
    const result = await getKey(key);
    setOutput((prev) => [
      ...prev,
      `> GET ${key}`,
      result.value ? `"${result.value}" (from ${result.from})` : "(nil)",
    ]);
  };

  return (
    <div className="bg-zinc-900 border border-zinc-800 rounded-xl p-6">
      <h2 className="text-lg font-bold text-white mb-4 font-mono">Console</h2>

      {/* Output */}
      <div className="bg-black rounded-lg p-4 h-48 overflow-y-auto mb-4 font-mono text-sm">
        {output.map((line, i) => (
          <div key={i} className={line.startsWith(">") ? "text-emerald-400" : "text-zinc-400"}>
            {line}
          </div>
        ))}
      </div>

      {/* Input */}
      <div className="flex gap-2">
        <input
          type="text"
          placeholder="key"
          value={key}
          onChange={(e) => setKeyInput(e.target.value)}
          className="flex-1 bg-zinc-800 border border-zinc-700 rounded-lg px-4 py-2 text-white font-mono"
        />
        <input
          type="text"
          placeholder="value"
          value={value}
          onChange={(e) => setValue(e.target.value)}
          className="flex-1 bg-zinc-800 border border-zinc-700 rounded-lg px-4 py-2 text-white font-mono"
        />
        <button onClick={handleSet} className="px-4 py-2 bg-emerald-500 text-black font-bold rounded-lg hover:bg-emerald-400">
          SET
        </button>
        <button onClick={handleGet} className="px-4 py-2 bg-blue-500 text-white font-bold rounded-lg hover:bg-blue-400">
          GET
        </button>
      </div>
    </div>
  );
}