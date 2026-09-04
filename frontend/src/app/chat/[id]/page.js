"use client";

import { useState, useEffect, useRef } from "react";
import { useParams, useRouter } from "next/navigation";
import { formatDistanceToNow } from "date-fns";
import toast from "react-hot-toast";

export default function ChatDetailPage() {
  const params = useParams();
  const router = useRouter();
  const userId = params.id;
  const [messages, setMessages] = useState([]); // ✅ Initialize as empty array
  const [newMessage, setNewMessage] = useState("");
  const [loading, setLoading] = useState(true);
  const [otherUser, setOtherUser] = useState(null);
  const [isConnected, setIsConnected] = useState(false);
  const [sending, setSending] = useState(false);
  const messagesEndRef = useRef(null);
  const wsRef = useRef(null);

  // Connect WebSocket
  useEffect(() => {
    const connectWebSocket = () => {
      const ws = new WebSocket("ws://localhost:8081/ws");
      wsRef.current = ws;

      ws.onopen = () => {
        console.log("WebSocket connected");
        setIsConnected(true);
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
      };

      ws.onerror = (error) => {
        console.error("WebSocket error:", error);
      };
    };

    const fetchChat = async () => {
      try {
        // Get other user info
        const userRes = await fetch(
          `http://localhost:8081/api/user/profile?id=${userId}`,
          {
            credentials: "include",
          },
        );
        if (userRes.ok) {
          const data = await userRes.json();
          setOtherUser(data);
        } else {
          toast.error("User not found");
          router.push("/chat");
          return;
        }

        // Get chat history
        const msgRes = await fetch(
          `http://localhost:8081/api/messages/history?user_id=${userId}`,
          {
            credentials: "include",
          },
        );
        if (msgRes.ok) {
          const data = await msgRes.json();
          // ✅ Ensure data is an array
          setMessages(Array.isArray(data) ? data : []);
        } else {
          setMessages([]);
        }
      } catch (error) {
        console.error("Error fetching chat:", error);
        setMessages([]);
        toast.error("Failed to load chat");
      } finally {
        setLoading(false);
      }
    };

    connectWebSocket();
    fetchChat();

    return () => {
      if (wsRef.current) {
        wsRef.current.close();
      }
    };
  }, [userId, router]);

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

    // Try WebSocket first
    if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
      const messageData = {
        type: "chat",
        data: JSON.stringify({
          recipient_id: parseInt(userId),
          content: content,
        }),
      };
      wsRef.current.send(JSON.stringify(messageData));

      // Add optimistic message
      const optimisticMsg = {
        id: Date.now(),
        sender_id: parseInt(userId),
        content: content,
        created_at: new Date().toISOString(),
        sender: { first_name: "You", last_name: "" },
      };
      setMessages((prev) => [...prev, optimisticMsg]);
      scrollToBottom();
      setSending(false);
      return;
    }

    const msgRes = await fetch(
      `http://localhost:8081/api/messages/history?user_id=${userId}`,
      {
        credentials: "include", // ✅ This is critical
      },
    );

    // Fallback: Use HTTP API
    try {
      const response = await fetch("http://localhost:8081/api/messages/send", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        credentials: "include",
        body: JSON.stringify({
          recipient_id: parseInt(userId),
          content: content,
        }),
      });

      if (!response.ok) {
        const data = await response.json();
        throw new Error(data.error || "Failed to send message");
      }

      const data = await response.json();

      // Add optimistic message
      const optimisticMsg = {
        id: data.message_id || Date.now(),
        sender_id: parseInt(userId),
        content: content,
        created_at: new Date().toISOString(),
        sender: { first_name: "You", last_name: "" },
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
      <div className='flex justify-center items-center h-64'>
        <div className='animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600'></div>
      </div>
    );
  }

  if (!otherUser) {
    return (
      <div className='max-w-4xl mx-auto py-12 px-4 text-center'>
        <h2 className='text-2xl font-bold text-gray-600'>User not found</h2>
        <button
          onClick={() => router.push("/chat")}
          className='mt-4 px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600'>
          Back to Chats
        </button>
      </div>
    );
  }

  return (
    <div className='max-w-4xl mx-auto py-4 px-4 h-[calc(100vh-12rem)]'>
      <div className='bg-white rounded-lg shadow h-full flex flex-col'>
        {/* Header */}
        <div className='p-4 border-b flex items-center'>
          <button
            onClick={() => router.push("/chat")}
            className='text-blue-500 hover:text-blue-700 mr-4'>
            ← Back
          </button>
          <div className='w-10 h-10 rounded-full bg-blue-500 flex items-center justify-center text-white font-bold'>
            {otherUser?.first_name?.[0] || "U"}
          </div>
          <div className='ml-3'>
            <h2 className='font-semibold'>
              {otherUser?.first_name} {otherUser?.last_name}
            </h2>
            <p
              className={`text-xs ${isConnected ? "text-green-500" : "text-gray-400"}`}>
              {isConnected ? "🟢 Online" : "⚪ Offline"}
            </p>
          </div>
        </div>

        {/* Messages */}
        <div className='flex-1 overflow-y-auto p-4 space-y-3'>
          {messages.length === 0 ? (
            <div className='text-center text-gray-400 py-8'>
              <p>No messages yet</p>
              <p className='text-sm'>Say hello!</p>
            </div>
          ) : (
            messages.map((msg, index) => {
              const isOwn = msg.sender_id === parseInt(userId);
              return (
                <div
                  key={msg.id || index}
                  className={`flex ${isOwn ? "justify-start" : "justify-end"}`}>
                  <div
                    className={`max-w-[70%] p-3 rounded-lg ${
                      isOwn
                        ? "bg-gray-200 text-gray-800"
                        : "bg-blue-500 text-white"
                    }`}>
                    {!isOwn && msg.sender && (
                      <p className='text-xs font-semibold text-blue-100 mb-1'>
                        {msg.sender.first_name}
                      </p>
                    )}
                    <p>{msg.content}</p>
                    <p
                      className={`text-xs mt-1 ${isOwn ? "text-gray-500" : "text-blue-100"}`}>
                      {formatDistanceToNow(new Date(msg.created_at), {
                        addSuffix: true,
                      })}
                    </p>
                  </div>
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
    </div>
  );
}
