"use client";

import { useState, useEffect, useRef } from "react";
import { useAuth } from "@/context/AuthContext";
import { formatDistanceToNow } from "date-fns";
import toast from "react-hot-toast";

export default function GroupChat({ groupId }) {
  const { user } = useAuth();
  const [messages, setMessages] = useState([]);
  const [newMessage, setNewMessage] = useState("");
  const [loading, setLoading] = useState(true);
  const [isConnected, setIsConnected] = useState(false);
  const [sending, setSending] = useState(false);
  const messagesEndRef = useRef(null);
  const wsRef = useRef(null);

  // Connect WebSocket
  useEffect(() => {
    const connectWebSocket = () => {
      const getCookie = (name) => {
        const value = `; ${document.cookie}`;
        const parts = value.split(`; ${name}=`);
        if (parts.length === 2) return parts.pop().split(";").shift();
        return null;
      };

      const token = getCookie("session_token");
      if (!token) {
        console.log("No session token found");
        return;
      }

      const ws = new WebSocket(`ws://localhost:8081/ws?token=${token}`);
      wsRef.current = ws;

      ws.onopen = () => {
        console.log("WebSocket connected");
        setIsConnected(true);
        // Join group room
        ws.send(
          JSON.stringify({
            type: "join_room",
            room_id: `group_${groupId}`,
          }),
        );
      };

      ws.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data);
          if (data.type === "chat") {
            const msg = JSON.parse(data.data);
            setMessages((prev) => [...prev, msg]);
            scrollToBottom();
          }
        } catch (error) {
          console.error("Error parsing message:", error);
        }
      };

      ws.onclose = () => {
        console.log("WebSocket disconnected");
        setIsConnected(false);
        setTimeout(connectWebSocket, 5000);
      };

      ws.onerror = (error) => {
        console.error("WebSocket error:", error);
        ws.close();
      };
    };

    const fetchMessages = async () => {
      try {
        const response = await fetch(
          `http://localhost:8081/api/group/messages?group_id=${groupId}`,
          {
            credentials: "include",
          },
        );
        if (response.ok) {
          const data = await response.json();
          setMessages(data);
        }
      } catch (error) {
        console.error("Error fetching group messages:", error);
      } finally {
        setLoading(false);
      }
    };

    connectWebSocket();
    fetchMessages();

    return () => {
      if (wsRef.current) {
        wsRef.current.close();
      }
    };
  }, [groupId]);

  useEffect(() => {
    scrollToBottom();
  }, [messages]);

  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  };

  const sendMessage = async (e) => {
    e.preventDefault();
    if (!newMessage.trim() || sending) return;

    setSending(true);
    const content = newMessage.trim();
    setNewMessage("");

    try {
      // Use WebSocket if connected
      if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
        const messageData = {
          type: "chat",
          data: JSON.stringify({
            group_id: parseInt(groupId),
            content: content,
          }),
        };
        wsRef.current.send(JSON.stringify(messageData));

        // Add optimistic message
        const optimisticMsg = {
          id: Date.now(),
          sender_id: user.id,
          content: content,
          created_at: new Date().toISOString(),
          sender: user,
          is_group_message: true,
        };
        setMessages((prev) => [...prev, optimisticMsg]);
        scrollToBottom();
        setSending(false);
        return;
      }

      // Fallback: Use HTTP API
      const response = await fetch(
        "http://localhost:8081/api/group/messages/send",
        {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
          },
          credentials: "include",
          body: JSON.stringify({
            group_id: parseInt(groupId),
            content: content,
          }),
        },
      );

      if (!response.ok) {
        const errorData = await response.json();
        throw new Error(errorData.error || "Failed to send message");
      }

      const data = await response.json();

      // Add optimistic message
      const optimisticMsg = {
        id: data.message_id || Date.now(),
        sender_id: user.id,
        content: content,
        created_at: new Date().toISOString(),
        sender: user,
        is_group_message: true,
      };
      setMessages((prev) => [...prev, optimisticMsg]);
      scrollToBottom();
    } catch (error) {
      console.error("Error sending message:", error);
      toast.error(error.message || "Failed to send message");
      setNewMessage(content);
    } finally {
      setSending(false);
    }
  };

  if (loading) {
    return (
      <div className='flex justify-center items-center h-32'>
        <div className='animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600'></div>
      </div>
    );
  }

  return (
    <div className='bg-white rounded-lg shadow h-[400px] flex flex-col'>
      {/* Messages */}
      <div className='flex-1 overflow-y-auto p-4 space-y-3'>
        {messages.length === 0 ? (
          <div className='text-center text-gray-400 py-8'>
            <p>No messages yet</p>
            <p className='text-sm'>Start the conversation!</p>
          </div>
        ) : (
          messages.map((msg, index) => {
            const isOwn = msg.sender_id === user.id;
            return (
              <div
                key={msg.id || index}
                className={`flex ${isOwn ? "justify-end" : "justify-start"}`}>
                <div className={`max-w-[70%] ${isOwn ? "order-2" : "order-1"}`}>
                  {!isOwn && msg.sender && (
                    <p className='text-xs font-semibold text-gray-600 mb-1'>
                      {msg.sender.first_name} {msg.sender.last_name}
                    </p>
                  )}
                  <div
                    className={`rounded-lg px-4 py-2 ${
                      isOwn
                        ? "bg-blue-500 text-white"
                        : "bg-gray-200 text-gray-800"
                    }`}>
                    <p className='break-words whitespace-pre-wrap'>
                      {msg.content}
                    </p>
                  </div>
                  <div
                    className={`flex items-center space-x-1 mt-1 ${isOwn ? "justify-end" : "justify-start"}`}>
                    <span className='text-xs text-gray-400'>
                      {formatDistanceToNow(new Date(msg.created_at), {
                        addSuffix: true,
                      })}
                    </span>
                  </div>
                </div>
                {!isOwn && msg.sender && (
                  <div className='order-1 mr-2'>
                    <div className='w-8 h-8 rounded-full bg-blue-500 flex items-center justify-center text-white text-xs font-bold'>
                      {msg.sender.first_name?.[0]}
                      {msg.sender.last_name?.[0]}
                    </div>
                  </div>
                )}
              </div>
            );
          })
        )}
        <div ref={messagesEndRef} />
      </div>

      {/* Input */}
      <form onSubmit={sendMessage} className='p-4 border-t flex space-x-2'>
        <input
          type='text'
          value={newMessage}
          onChange={(e) => setNewMessage(e.target.value)}
          placeholder={isConnected ? "Type a message..." : "Connecting..."}
          className='flex-1 px-4 py-2 border rounded-full focus:outline-none focus:ring-2 focus:ring-blue-500'
          disabled={!isConnected && !sending}
        />
        <button
          type='submit'
          disabled={!newMessage.trim() || sending || !isConnected}
          className='px-6 py-2 bg-blue-500 text-white rounded-full hover:bg-blue-600 transition disabled:opacity-50'>
          {sending ? "Sending..." : "Send"}
        </button>
      </form>
    </div>
  );
}
