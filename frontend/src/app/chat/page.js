"use client";

import { useState, useEffect } from "react";
import { useRouter } from "next/navigation";

export default function ChatPage() {
  const router = useRouter();
  const [chats, setChats] = useState([]); // ✅ Initialize as empty array
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const fetchChats = async () => {
      try {
        const authRes = await fetch("http://localhost:8081/api/check-auth", {
          credentials: "include",
        });
        const authData = await authRes.json();
        if (!authData.authenticated) {
          router.push("/login");
          return;
        }

        // For now, use mock data until backend is ready
        const mockChats = [
          {
            id: 1,
            name: "John Doe",
            lastMessage: "Hey there!",
            time: "10:30 AM",
            unread: 2,
          },
          {
            id: 2,
            name: "Jane Smith",
            lastMessage: "See you tomorrow",
            time: "9:15 AM",
            unread: 0,
          },
          {
            id: 3,
            name: "Group Chat",
            lastMessage: "Great idea!",
            time: "Yesterday",
            unread: 5,
          },
        ];
        setChats(mockChats);
      } catch (error) {
        console.error("Error fetching chats:", error);
        setChats([]);
      } finally {
        setLoading(false);
      }
    };

    fetchChats();
  }, [router]);

  if (loading) {
    return (
      <div className='flex justify-center items-center h-64'>
        <div className='animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600'></div>
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
                key={chat.id}
                onClick={() => router.push(`/chat/${chat.id}`)}
                className='p-4 hover:bg-gray-50 cursor-pointer transition flex items-center justify-between'>
                <div className='flex items-center space-x-3'>
                  <div className='w-12 h-12 rounded-full bg-blue-500 flex items-center justify-center text-white font-bold text-lg'>
                    {chat.name.charAt(0)}
                  </div>
                  <div>
                    <p className='font-semibold'>{chat.name}</p>
                    <p className='text-sm text-gray-500'>{chat.lastMessage}</p>
                  </div>
                </div>
                <div className='text-right'>
                  <p className='text-xs text-gray-400'>{chat.time}</p>
                  {chat.unread > 0 && (
                    <span className='inline-block mt-1 bg-blue-500 text-white text-xs font-bold px-2 py-1 rounded-full'>
                      {chat.unread}
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
