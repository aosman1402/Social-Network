"use client";

import { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import { apiFetch } from "@/lib/api";
import LoadingSpinner from "@/components/LoadingSpinner";
import { formatDistanceToNow } from "@/lib/formatDistanceToNow";
import { useRequireAuth } from "@/hooks/useRequireAuth";
import { useWebSocket } from "@/context/WebSocketContext";

export default function ChatPage() {
  const router = useRouter();
  const { user, loading: authLoading } = useRequireAuth();
  const { subscribe } = useWebSocket();
  const [chats, setChats] = useState([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!user) return;

    const fetchChats = async () => {
      try {
        const { ok, data } = await apiFetch("/chats");
        setChats(ok && Array.isArray(data) ? data : []);
      } catch (error) {
        console.error("Error fetching chats:", error);
        setChats([]);
      } finally {
        setLoading(false);
      }
    };

    fetchChats();
  }, [user]);

  // Previously this list only ever loaded once on mount - a message
  // arriving while it was open didn't move that conversation to the top,
  // update its preview/unread badge, or add a brand-new conversation row,
  // so nothing on this page ever visibly changed in real time.
  useEffect(() => {
    if (!user) return;

    return subscribe("chat", (msg) => {
      if (msg.group_id) return; // this list only holds direct conversations
      const otherUserId = msg.sender_id === user.id ? msg.recipient_id : msg.sender_id;
      const isIncoming = msg.sender_id !== user.id;

      setChats((prev) => {
        const existing = prev.find((c) => c.other_user_id === otherUserId);
        const updated = {
          other_user_id: otherUserId,
          other_user: existing?.other_user || msg.sender,
          last_message: msg.content,
          last_message_at: msg.created_at,
          unread_count: isIncoming
            ? (existing?.unread_count || 0) + 1
            : existing?.unread_count || 0,
        };
        const rest = prev.filter((c) => c.other_user_id !== otherUserId);
        return [updated, ...rest];
      });
    });
  }, [subscribe, user]);

  if (authLoading || loading) {
    return (
      <div className='flex justify-center items-center h-64'>
        <LoadingSpinner size='large' />
      </div>
    );
  }

  return (
    <div className='max-w-3xl mx-auto py-4 px-4'>
      <div className='bg-white rounded-lg shadow'>
        <div className='p-4 border-b'>
          <h1 className='text-2xl font-bold'>Messages</h1>
        </div>

        <div className='divide-y'>
          {chats.length === 0 ? (
            <div className='p-8 text-center text-gray-500'>
              <p>No conversations yet</p>
            </div>
          ) : (
            chats.map((chat) => (
              <div
                key={chat.other_user_id}
                onClick={() => router.push(`/chat/${chat.other_user_id}`)}
                className='p-4 hover:bg-gray-50 cursor-pointer transition flex items-center justify-between'>
                <div className='flex items-center space-x-3'>
                  <div className='w-12 h-12 rounded-full bg-blue-500 flex items-center justify-center text-white font-bold text-lg'>
                    {chat.other_user?.first_name?.[0] || "U"}
                  </div>
                  <div>
                    <p className='font-semibold'>
                      {chat.other_user?.first_name} {chat.other_user?.last_name}
                    </p>
                    <p className='text-sm text-gray-500'>
                      {chat.last_message || "No messages yet"}
                    </p>
                  </div>
                </div>
                <div className='text-right'>
                  {chat.last_message_at && (
                    <p className='text-xs text-gray-400'>
                      {formatDistanceToNow(new Date(chat.last_message_at), {
                        addSuffix: true,
                      })}
                    </p>
                  )}
                  {chat.unread_count > 0 && (
                    <span className='inline-block mt-1 bg-blue-500 text-white text-xs font-bold px-2 py-1 rounded-full'>
                      {chat.unread_count}
                    </span>
                  )}
                </div>
              </div>
            ))
          )}
        </div>
      </div>
    </div>
  );
}
