import { NextRequest, NextResponse } from "next/server";
import net from "net";

export async function GET(req: NextRequest) {
  const port = req.nextUrl.searchParams.get("port");
  const key = req.nextUrl.searchParams.get("key");

  if (!port || !key) {
    return NextResponse.json({ error: "Missing params" }, { status: 400 });
  }

  return new Promise((resolve) => {
    const client = new net.Socket();
    client.setTimeout(2000);

    client.connect(parseInt(port), "localhost", () => {
      client.write(`GET ${key}\n`);
    });

    client.on("data", (data) => {
      const response = data.toString().trim();
      client.destroy();
      resolve(NextResponse.json({ 
        value: response === "(nil)" ? null : response 
      }));
    });

    client.on("error", () => {
      resolve(NextResponse.json({ value: null, error: "Connection failed" }));
    });

    client.on("timeout", () => {
      client.destroy();
      resolve(NextResponse.json({ value: null, error: "Timeout" }));
    });
  });
}