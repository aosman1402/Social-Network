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
  const [isConnected, setIsConnected] = useState(false);
  const wsRef = useRef(null);
  const listenersRef = useRef(new Map()); // event type -> Set<handler>
  const reconnectTimerRef = useRef(null);
  const reconnectDelayRef = useRef(1000);
  const unmountingRef = useRef(false);

  useEffect(() => {
    if (!user) {
      return;
    }

    unmountingRef.current = false;

    const connect = () => {
      const ws = new WebSocket("ws://localhost:8081/ws");
      wsRef.current = ws;

      ws.onopen = () => {
        setIsConnected(true);
        reconnectDelayRef.current = 1000; // back to the short delay once healthy
      };

      ws.onmessage = (event) => {
        let envelope;
        try {
          envelope = JSON.parse(event.data);
        } catch (error) {
          console.error("Error parsing WebSocket message:", error);
          return;
        }

        const handlers = listenersRef.current.get(envelope.type);
        if (handlers) {
          handlers.forEach((handler) => handler(envelope.data));
        }
      };

      ws.onclose = () => {
        setIsConnected(false);
        if (unmountingRef.current) return;
        // Reconnect with exponential backoff (capped at 30s) instead of
        // leaving the app silently offline until the next page visit.
        const maxReconnectDelay = 30000;
        reconnectTimerRef.current = setTimeout(() => {
          reconnectDelayRef.current = Math.min(
            reconnectDelayRef.current * 2,
            maxReconnectDelay,
          );
          connect();
        }, reconnectDelayRef.current);
      };

      ws.onerror = (error) => {
        console.error("WebSocket error:", error);
      };
    };

    connect();

    return () => {
      unmountingRef.current = true;
      clearTimeout(reconnectTimerRef.current);
      const ws = wsRef.current;
      if (ws) {
        if (ws.readyState === WebSocket.CONNECTING) {
          // Closing mid-handshake logs a "closed before connection
          // established" warning; wait for it to open, then close it.
          ws.addEventListener("open", () => ws.close(), { once: true });
        } else {
          ws.close();
        }
      }
      wsRef.current = null;
      setIsConnected(false);
    };
  }, [user]);

  const send = useCallback((message) => {
    const ws = wsRef.current;
    if (!ws || ws.readyState !== WebSocket.OPEN) return false;
    ws.send(JSON.stringify(message));
    return true;
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
