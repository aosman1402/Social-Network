// One self-reconnecting WebSocket connection, independent of React so its
// lifecycle can be unit-tested (see socketConnection.test.js).
//
// Every piece of state lives in this closure, per connection. The previous
// version kept a single `unmountingRef` flag shared by every run of the
// provider's effect: when the effect re-ran (React Strict Mode's double
// mount in dev, or the `user` object changing after login or a profile
// save), the old socket's late onclose saw the flag already reset by the
// new run and "reconnected" - leaving two live sockets in one tab. The
// server sends each event to every socket a user has open, so every chat
// message then arrived, and was shown, twice.
//
// Here, close() permanently stops this connection, and events from any
// socket other than the current one are ignored, so there is never more
// than one live socket per connection.
export function createSocketConnection({
  url,
  onMessage,
  onStatusChange,
  WebSocketImpl = globalThis.WebSocket,
  initialDelay = 1000,
  maxDelay = 30000,
}) {
  let ws = null;
  let closed = false;
  let reconnectTimer = null;
  let delay = initialDelay;

  const connect = () => {
    if (closed) return;
    const socket = new WebSocketImpl(url);
    ws = socket;

    socket.onopen = () => {
      if (socket !== ws) return;
      delay = initialDelay; // back to the short delay once healthy
      onStatusChange?.(true);
    };

    socket.onmessage = (event) => {
      if (socket !== ws) return;
      let envelope;
      try {
        envelope = JSON.parse(event.data);
      } catch (error) {
        console.error("Error parsing WebSocket message:", error);
        return;
      }
      onMessage?.(envelope);
    };

    socket.onclose = () => {
      if (socket !== ws) return; // a replaced socket - nothing to do
      ws = null;
      onStatusChange?.(false);
      if (closed) return;
      // Reconnect with exponential backoff (capped) instead of leaving the
      // app silently offline until the next page visit.
      reconnectTimer = setTimeout(() => {
        reconnectTimer = null;
        delay = Math.min(delay * 2, maxDelay);
        connect();
      }, delay);
    };

    socket.onerror = () => {
      if (socket !== ws) return;
      // A warning, not an error: a dropped connection (server restart,
      // network blip) is expected and handled - onclose reconnects with
      // backoff. console.error would show as a red error in Next.js dev.
      console.warn("WebSocket connection lost - reconnecting…");
    };
  };

  connect();

  return {
    send(message) {
      if (!ws || ws.readyState !== WebSocketImpl.OPEN) return false;
      ws.send(JSON.stringify(message));
      return true;
    },

    close() {
      if (closed) return;
      closed = true;
      clearTimeout(reconnectTimer);
      const socket = ws;
      ws = null; // from here on, every handler above ignores this socket
      onStatusChange?.(false);
      if (!socket) return;
      if (socket.readyState === WebSocketImpl.CONNECTING) {
        // Closing mid-handshake logs a "closed before connection
        // established" warning; wait for it to open, then close it.
        socket.addEventListener?.("open", () => socket.close(), { once: true });
      } else {
        socket.close();
      }
    },
  };
}
