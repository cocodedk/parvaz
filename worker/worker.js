// Parvaz Cloudflare Worker — raw TCP passthrough over WebSocket.
//
// Deployment contract: a single /tunnel endpoint accepts a WebSocket
// upgrade with ?k=<access_key>&host=<target>&port=<target_port>. On auth
// success, the Worker opens an outbound TCP socket via cloudflare:sockets
// and pipes bytes bidirectionally. Bytes pass through opaque — HTTPS,
// IMAP, SSH, whatever.
//
// The access key is compile-time: set ACCESS_KEY below to a strong random
// value and redeploy when rotating. For production, move it to an env
// binding (wrangler secret put ACCESS_KEY).

import { connect } from "cloudflare:sockets";

const ACCESS_KEY = "CHANGE_ME_TO_A_STRONG_SECRET";

export default {
  async fetch(request) {
    const url = new URL(request.url);
    if (url.pathname !== "/tunnel") {
      return new Response("parvaz relay", { status: 200 });
    }
    if (request.headers.get("Upgrade") !== "websocket") {
      return new Response("expected websocket upgrade", { status: 426 });
    }

    const key = url.searchParams.get("k") || "";
    const host = url.searchParams.get("host") || "";
    const port = parseInt(url.searchParams.get("port") || "0", 10);

    if (key !== ACCESS_KEY) {
      return new Response("unauthorized", { status: 401 });
    }
    if (!host || port <= 0 || port > 65535) {
      return new Response("missing/invalid host or port", { status: 400 });
    }

    const [client, server] = Object.values(new WebSocketPair());
    server.accept();

    // Open upstream TCP. Errors are surfaced back via WS close(1011).
    let socket;
    try {
      socket = connect({ hostname: host, port });
    } catch (err) {
      server.close(1011, "connect failed: " + err.message);
      return new Response(null, { status: 101, webSocket: client });
    }

    const writer = socket.writable.getWriter();
    const reader = socket.readable.getReader();

    // WebSocket -> upstream TCP
    server.addEventListener("message", async (ev) => {
      try {
        await writer.write(ev.data);
      } catch (err) {
        server.close(1011, "ws→tcp write: " + err.message);
      }
    });

    // Upstream TCP -> WebSocket
    (async () => {
      try {
        while (true) {
          const { value, done } = await reader.read();
          if (done) break;
          server.send(value);
        }
        server.close(1000, "upstream closed");
      } catch (err) {
        server.close(1011, "tcp→ws read: " + err.message);
      }
    })();

    server.addEventListener("close", () => {
      try { socket.close(); } catch {}
    });

    return new Response(null, { status: 101, webSocket: client });
  },
};
