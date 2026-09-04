"use client";

import { useContext, useCallback, useEffect, useRef } from "react";
import { SocketContext } from "@/context/SocketContext";

/**
 * Custom hook for accessing WebSocket functionality
 * @returns {Object} Socket context value and helper methods
 * @throws {Error} If used outside of SocketProvider
 */
export const useSocket = () => {
  const context = useContext(SocketContext);

  if (!context) {
    throw new Error("useSocket must be used within a SocketProvider");
  }

  const { socket, isConnected } = context;

  /**
   * Send a message through the socket
   * @param {string} event - Event name
   * @param {*} data - Data to send
   */
  const emit = useCallback(
    (event, data) => {
      if (socket && isConnected) {
        socket.emit(event, data);
        return true;
      }
      console.warn("Socket not connected, message not sent:", event);
      return false;
    },
    [socket, isConnected],
  );

  /**
   * Register event listener
   * @param {string} event - Event name
   * @param {Function} callback - Callback function
   */
  const on = useCallback(
    (event, callback) => {
      if (socket) {
        socket.on(event, callback);
      }
    },
    [socket],
  );

  /**
   * Remove event listener
   * @param {string} event - Event name
   * @param {Function} callback - Callback function (optional)
   */
  const off = useCallback(
    (event, callback) => {
      if (socket) {
        if (callback) {
          socket.off(event, callback);
        } else {
          socket.off(event);
        }
      }
    },
    [socket],
  );

  /**
   * Send a chat message
   * @param {Object} data - Message data
   * @param {number} data.recipient_id - Recipient user ID
   * @param {number} data.group_id - Group ID (optional)
   * @param {string} data.content - Message content
   */
  const sendMessage = useCallback(
    (data) => {
      return emit("chat", data);
    },
    [emit],
  );

  /**
   * Send typing indicator
   * @param {Object} data - Typing data
   * @param {number} data.recipient_id - Recipient user ID
   * @param {number} data.group_id - Group ID (optional)
   * @param {boolean} data.is_typing - Whether user is typing
   */
  const sendTyping = useCallback(
    (data) => {
      return emit("typing", data);
    },
    [emit],
  );

  /**
   * Join a room
   * @param {string} roomId - Room ID to join
   */
  const joinRoom = useCallback(
    (roomId) => {
      return emit("join_room", { room_id: roomId });
    },
    [emit],
  );

  /**
   * Leave a room
   * @param {string} roomId - Room ID to leave
   */
  const leaveRoom = useCallback(
    (roomId) => {
      return emit("leave_room", { room_id: roomId });
    },
    [emit],
  );

  /**
   * Send read receipt
   * @param {number} messageId - Message ID
   */
  const sendReadReceipt = useCallback(
    (messageId) => {
      return emit("read_receipt", { message_id: messageId });
    },
    [emit],
  );

  /**
   * Send ping to keep connection alive
   */
  const ping = useCallback(() => {
    return emit("ping", {});
  }, [emit]);

  return {
    // Core socket state
    socket,
    isConnected,

    // Core methods
    emit,
    on,
    off,

    // Chat methods
    sendMessage,
    sendTyping,
    sendReadReceipt,

    // Room methods
    joinRoom,
    leaveRoom,

    // Utility methods
    ping,
  };
};

export default useSocket;
