import { createSocketConnection } from "./socketConnection";

// Minimal stand-in for the browser WebSocket: tests drive open/close/message
// by hand, and every instance is recorded so we can count live sockets.
class FakeWebSocket {
  static CONNECTING = 0;
  static OPEN = 1;
  static CLOSED = 3;
  static instances = [];

  constructor(url) {
    this.url = url;
    this.readyState = FakeWebSocket.CONNECTING;
    this.sent = [];
    this.openListeners = [];
    FakeWebSocket.instances.push(this);
  }

  addEventListener(type, fn) {
    if (type === "open") this.openListeners.push(fn);
  }

  send(data) {
    this.sent.push(data);
  }

  close() {
    if (this.readyState === FakeWebSocket.CLOSED) return;
    this.readyState = FakeWebSocket.CLOSED;
    // Real sockets fire onclose asynchronously; tests call fireClose().
    this.closeRequested = true;
  }

  // --- test controls ---
  fireOpen() {
    this.readyState = FakeWebSocket.OPEN;
    this.onopen?.();
    this.openListeners.forEach((fn) => fn());
  }

  fireClose() {
    this.readyState = FakeWebSocket.CLOSED;
    this.onclose?.();
  }

  fireMessage(envelope) {
    this.onmessage?.({ data: JSON.stringify(envelope) });
  }
}

const liveSockets = () =>
  FakeWebSocket.instances.filter((s) => s.readyState !== FakeWebSocket.CLOSED);

const connect = (overrides = {}) => {
  const received = [];
  const statuses = [];
  const conn = createSocketConnection({
    url: "ws://test/ws",
    WebSocketImpl: FakeWebSocket,
    onMessage: (env) => received.push(env),
    onStatusChange: (s) => statuses.push(s),
    ...overrides,
  });
  return { conn, received, statuses };
};

beforeEach(() => {
  FakeWebSocket.instances = [];
  jest.useFakeTimers();
});

afterEach(() => {
  jest.useRealTimers();
});

// The bug: React's effect runs, is cleaned up, and runs again (Strict Mode
// in dev, or the user object changing). The first socket closes late, and
// used to "reconnect" because a shared flag had been reset by the second
// run - two live sockets, every message delivered twice.
test("a closed connection's late onclose does not open a second socket", () => {
  const first = connect();
  const ws1 = FakeWebSocket.instances[0];
  first.conn.close(); // effect cleanup while ws1 is still connecting

  const second = connect(); // effect runs again
  const ws2 = FakeWebSocket.instances[1];
  ws2.fireOpen();

  ws1.fireOpen(); // ws1 finishes its handshake - close() closes it now
  expect(ws1.closeRequested).toBe(true);
  ws1.fireClose(); // ...and its onclose arrives late
  jest.runAllTimers();

  expect(FakeWebSocket.instances).toHaveLength(2); // no ws3
  expect(liveSockets()).toEqual([ws2]);

  ws2.fireMessage({ type: "chat", data: { id: 1 } });
  expect(second.received).toHaveLength(1);
  expect(first.received).toHaveLength(0);
});

test("the same race with an already-open socket", () => {
  const first = connect();
  const ws1 = FakeWebSocket.instances[0];
  ws1.fireOpen();
  first.conn.close();

  const second = connect();
  FakeWebSocket.instances[1].fireOpen();
  ws1.fireClose(); // late onclose from the old socket
  jest.runAllTimers();

  expect(FakeWebSocket.instances).toHaveLength(2);
  expect(liveSockets()).toHaveLength(1);
  expect(second.statuses.at(-1)).toBe(true);
});

test("messages from a replaced socket are ignored", () => {
  const { conn, received } = connect();
  const ws1 = FakeWebSocket.instances[0];
  ws1.fireOpen();
  ws1.fireClose(); // dropped connection
  jest.advanceTimersByTime(1000);
  const ws2 = FakeWebSocket.instances[1];
  ws2.fireOpen();

  ws1.fireMessage({ type: "chat", data: { id: 1 } }); // stale socket
  ws2.fireMessage({ type: "chat", data: { id: 2 } });
  expect(received.map((e) => e.data.id)).toEqual([2]);
  conn.close();
});

test("reconnects with exponential backoff after an unexpected close", () => {
  const { conn, statuses } = connect();
  FakeWebSocket.instances[0].fireOpen();
  FakeWebSocket.instances[0].fireClose();
  expect(statuses).toEqual([true, false]);

  jest.advanceTimersByTime(999);
  expect(FakeWebSocket.instances).toHaveLength(1);
  jest.advanceTimersByTime(1);
  expect(FakeWebSocket.instances).toHaveLength(2);

  // Fails again before opening: the next wait doubles.
  FakeWebSocket.instances[1].fireClose();
  jest.advanceTimersByTime(1999);
  expect(FakeWebSocket.instances).toHaveLength(2);
  jest.advanceTimersByTime(1);
  expect(FakeWebSocket.instances).toHaveLength(3);
  conn.close();
});

test("close() stops a pending reconnect", () => {
  const { conn } = connect();
  FakeWebSocket.instances[0].fireOpen();
  FakeWebSocket.instances[0].fireClose();
  conn.close();
  jest.runAllTimers();
  expect(FakeWebSocket.instances).toHaveLength(1);
});

test("send() only works while open", () => {
  const { conn } = connect();
  const ws = FakeWebSocket.instances[0];
  expect(conn.send({ type: "ping" })).toBe(false);
  ws.fireOpen();
  expect(conn.send({ type: "ping" })).toBe(true);
  expect(ws.sent).toEqual([JSON.stringify({ type: "ping" })]);
  conn.close();
  expect(conn.send({ type: "ping" })).toBe(false);
});
