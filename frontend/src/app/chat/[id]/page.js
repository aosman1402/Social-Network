"use client";

import { useState, useEffect, useRef } from "react";
import { useParams, useRouter } from "next/navigation";
import { formatDistanceToNow } from "@/lib/formatDistanceToNow";
import { toast } from "@/context/ToastContext";
import { apiFetch } from "@/lib/api";
import LoadingSpinner from "@/components/LoadingSpinner";
import { useAuth } from "@/context/AuthContext";
import { useWebSocket } from "@/context/WebSocketContext";

export default function ChatDetailPage() {
  const params = useParams();
  const router = useRouter();
  const userId = params.id; // the OTHER person in this conversation
  const { user } = useAuth();
  const { isConnected, send, subscribe } = useWebSocket();
  const [messages, setMessages] = useState([]);
  const [newMessage, setNewMessage] = useState("");
  const [loading, setLoading] = useState(true);
  const [otherUser, setOtherUser] = useState(null);
  const [sending, setSending] = useState(false);
  // Set when the backend confirms these two users aren't allowed to message
  // each other (no follow relationship in either direction) - blocks the
  // compose UI instead of silently showing an empty conversation.
  const [blocked, setBlocked] = useState(false);
  const messagesEndRef = useRef(null);

  // Load the other user's profile and this conversation's history.
  useEffect(() => {
    const fetchChat = async () => {
      try {
        const { ok: userOk, data: userData } = await apiFetch(
          `/user/profile?id=${userId}`,
        );
        if (userOk) {
          setOtherUser(userData);
        } else {
          toast.error("User not found");
          router.push("/chat");
          return;
        }

        const {
          ok: msgOk,
          status: msgStatus,
          data: msgData,
        } = await apiFetch(`/messages/history?user_id=${userId}`);

        if (!msgOk && msgStatus === 403) {
          setBlocked(true);
          setMessages([]);
          return;
        }

        setMessages(msgOk && Array.isArray(msgData) ? msgData : []);
      } catch (error) {
        console.error("Error fetching chat:", error);
        setMessages([]);
        toast.error("Failed to load chat");
      } finally {
        setLoading(false);
      }
    };

    fetchChat();
  }, [userId, router]);

  // Live-append incoming messages that belong to this conversation. The
  // shared socket delivers every "chat" event addressed to this user, not
  // just ones from the person currently open here, so it has to be
  // filtered by conversation partner.
  useEffect(() => {
    const otherUserId = parseInt(userId);
    return subscribe("chat", (msg) => {
      if (msg.sender_id !== otherUserId && msg.recipient_id !== otherUserId) {
        return;
      }
      setMessages((prev) => [...prev, msg]);
    });
  }, [subscribe, userId]);

  // A WebSocket "chat" send is optimistic on the frontend (see sendMessage
  // below) - if the server rejects it (e.g. no follow relationship), the
  // sender otherwise never finds out and the message stays shown as if it
  // went through. Roll back the optimistic entry and surface why.
  useEffect(() => {
    const otherUserId = parseInt(userId);
    return subscribe("chat_error", (err) => {
      if (err.recipient_id !== otherUserId) return;
      setMessages((prev) => prev.filter((m) => !m.pending));
      toast.error(err.error || "Failed to send message");
    });
  }, [subscribe, userId]);

  useEffect(() => {
    scrollToBottom();
  }, [messages]);

  // Tell the server which of the other person's messages we've now seen.
  // Re-runs whenever the message list changes or the socket (re)connects;
  // marking an already-read message again is a harmless no-op server-side.
  useEffect(() => {
    if (!isConnected) return;
    const otherUserId = parseInt(userId);
    messages
      .filter((m) => m.sender_id === otherUserId)
      .forEach((m) => {
        send({ type: "read_receipt", data: { message_id: m.id } });
      });
  }, [isConnected, messages, userId, send]);

  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  };

  const sendMessage = async (e) => {
    e.preventDefault();
    if (!newMessage.trim() || sending) return;

    setSending(true);
    const content = newMessage.trim();
    setNewMessage("");

    const optimisticMsg = {
      id: Date.now(),
      sender_id: user.id,
      content,
      created_at: new Date().toISOString(),
      sender: { first_name: "You", last_name: "" },
      // Cleared once the send is confirmed - a chat_error event for this
      // recipient removes any message still flagged pending (see the
      // chat_error subscription above), since the WS send below doesn't
      // wait for a server response before showing "sent".
      pending: true,
    };

    // Try WebSocket first
    if (
      send({
        type: "chat",
        data: { recipient_id: parseInt(userId), content },
      })
    ) {
      setMessages((prev) => [...prev, optimisticMsg]);
      setSending(false);
      return;
    }

    // Fallback: Use HTTP API
    try {
      const { ok, data } = await apiFetch("/messages/send", {
        method: "POST",
        json: { recipient_id: parseInt(userId), content },
      });

      if (!ok) {
        throw new Error(data.error || "Failed to send message");
      }

      setMessages((prev) => [
        ...prev,
        { ...optimisticMsg, id: data.message_id || optimisticMsg.id },
      ]);
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
        <LoadingSpinner size='large' />
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

  // Neither user follows the other - direct messages require a follow
  // relationship in either direction, so there's no conversation to show.
  if (blocked) {
    return (
      <div className='max-w-md mx-auto py-12 px-4 text-center'>
        <div className='w-16 h-16 rounded-full bg-blue-500 flex items-center justify-center text-white text-xl font-bold mx-auto'>
          {otherUser?.first_name?.[0] || "U"}
        </div>
        <h2 className='text-xl font-bold mt-4'>
          {otherUser?.first_name} {otherUser?.last_name}
        </h2>
        <p className='text-gray-600 mt-3'>
          🔒 You can't message {otherUser?.first_name} yet. You can only send
          direct messages to people who follow you or that you follow.
        </p>
        <div className='mt-4 flex justify-center gap-2'>
          <button
            onClick={() => router.push(`/profile/${userId}`)}
            className='px-4 py-2 bg-blue-500 text-white rounded-lg hover:bg-blue-600 transition'>
            View Profile
          </button>
          <button
            onClick={() => router.push("/chat")}
            className='px-4 py-2 bg-gray-200 text-gray-700 rounded-lg hover:bg-gray-300 transition'>
            Back to Chats
          </button>
        </div>
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
              const isTheirs = msg.sender_id === parseInt(userId);
              return (
                <div
                  key={msg.id || index}
                  className={`flex ${isTheirs ? "justify-start" : "justify-end"}`}>
                  <div
                    className={`max-w-[70%] p-3 rounded-lg ${
                      isTheirs
                        ? "bg-gray-200 text-gray-800"
                        : "bg-blue-500 text-white"
                    }`}>
                    {isTheirs && msg.sender && (
                      <p className='text-xs font-semibold text-blue-100 mb-1'>
                        {msg.sender.first_name}
                      </p>
                    )}
                    <p>{msg.content}</p>
                    <p
                      className={`text-xs mt-1 ${isTheirs ? "text-gray-500" : "text-blue-100"}`}>
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
            disabled={!newMessage.trim() || sending}
            className='px-6 py-2 bg-blue-500 text-white rounded-full hover:bg-blue-600 transition disabled:opacity-50'>
            {sending ? "Sending..." : "Send"}
          </button>
        </form>
      </div>
    </div>
  );
}
