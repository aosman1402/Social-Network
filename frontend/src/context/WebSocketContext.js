"use client";

import {
  createContext,
  useContext,
  useEffect,
  useRef,
  useCallback,
  useState,
} from "react";
import { useAuth } from "@/context/AuthContext";
import { createSocketConnection } from "@/lib/socketConnection";

const WebSocketContext = createContext(null);

// A single WebSocket connection for the whole app, shared via context.
// Previously chat/[id]/page.js and NotificationBell.js each opened and
// managed their own independent connection (with their own duplicated
// reconnect/backoff logic), so an authenticated tab held two live sockets
// instead of one.
//
// Every event the server sends is {"type": "...", "data": {...}} with data
// embedded as a raw JSON object (never a JSON-encoded string) - consumers
// call subscribe(type, handler) to receive just the events they care about.
export function WebSocketProvider({ children }) {
  const { user } = useAuth();
  const userId = user?.id;
  const [isConnected, setIsConnected] = useState(false);
  const connectionRef = useRef(null);
  const listenersRef = useRef(new Map()); // event type -> Set<handler>

  // Keyed on the user's id, not the user object: the object is replaced
  // after login and on every profile save, and each replacement used to
  // tear down and reopen the socket. All connection state (including
  // whether it's been closed) lives inside createSocketConnection, per
  // connection - see lib/socketConnection.js for the duplicate-socket bug
  // this replaced.
  useEffect(() => {
    if (!userId) return;

    const connection = createSocketConnection({
      url: "ws://localhost:8081/ws",
      onStatusChange: setIsConnected,
      onMessage: (envelope) => {
        const handlers = listenersRef.current.get(envelope.type);
        if (handlers) {
          handlers.forEach((handler) => handler(envelope.data));
        }
      },
    });
    connectionRef.current = connection;

    return () => {
      connection.close();
      if (connectionRef.current === connection) connectionRef.current = null;
    };
  }, [userId]);

  const send = useCallback((message) => {
    return connectionRef.current ? connectionRef.current.send(message) : false;
  }, []);

  // Registers handler for events of the given type. Returns an unsubscribe
  // function - call it from a useEffect cleanup.
  const subscribe = useCallback((type, handler) => {
    if (!listenersRef.current.has(type)) {
      listenersRef.current.set(type, new Set());
    }
    listenersRef.current.get(type).add(handler);
    return () => {
      listenersRef.current.get(type)?.delete(handler);
    };
  }, []);

  return (
    <WebSocketContext.Provider value={{ isConnected, send, subscribe }}>
      {children}
    </WebSocketContext.Provider>
  );
}

export function useWebSocket() {
  const context = useContext(WebSocketContext);
  if (!context) {
    throw new Error("useWebSocket must be used within a WebSocketProvider");
  }
  return context;
}
