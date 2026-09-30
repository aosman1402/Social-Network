"use client";

import { useState, useEffect, useMemo } from "react";
import { useRouter } from "next/navigation";
import { apiFetch } from "@/lib/api";
import LoadingSpinner from "@/components/LoadingSpinner";
import Avatar from "@/components/Avatar";
import { formatDistanceToNow } from "@/lib/formatDistanceToNow";
import { useRequireAuth } from "@/hooks/useRequireAuth";
import { useWebSocket } from "@/context/WebSocketContext";
import Icon from "@/components/Icon";

const fullName = (u) => `${u?.first_name || ""} ${u?.last_name || ""}`.trim();

export default function ChatPage() {
  const router = useRouter();
  const { user, loading: authLoading } = useRequireAuth();
  const { subscribe } = useWebSocket();
  const [chats, setChats] = useState([]);
  const [contacts, setContacts] = useState([]); // everyone we can message
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState("");

  useEffect(() => {
    if (!user) return;

    const fetchAll = async () => {
      try {
        const [chatRes, contactRes] = await Promise.all([
          apiFetch("/chats"),
          apiFetch("/chat/contacts"),
        ]);
        setChats(chatRes.ok && Array.isArray(chatRes.data) ? chatRes.data : []);
        setContacts(contactRes.ok && Array.isArray(contactRes.data) ? contactRes.data : []);
      } catch (error) {
        console.error("Error fetching chats:", error);
        setChats([]);
      } finally {
        setLoading(false);
      }
    };

    fetchAll();
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
          online: existing?.online,
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

  // Live online / offline dots.
  useEffect(() => {
    return subscribe("presence", ({ user_id, online }) => {
      setChats((prev) =>
        prev.map((c) => (c.other_user_id === user_id ? { ...c, online } : c)),
      );
      setContacts((prev) =>
        prev.map((c) => (c.user.id === user_id ? { ...c, online } : c)),
      );
    });
  }, [subscribe]);

  const query = search.trim().toLowerCase();
  const matches = (u) =>
    !query ||
    fullName(u).toLowerCase().includes(query) ||
    (u?.nickname || "").toLowerCase().includes(query);

  const shownChats = chats.filter((c) => matches(c.other_user));

  // People we can message but haven't yet - online first, then by name.
  const newChatContacts = useMemo(() => {
    const inChats = new Set(chats.map((c) => c.other_user_id));
    return contacts
      .filter((c) => !inChats.has(c.user.id))
      .sort((a, b) => (b.online - a.online) || fullName(a.user).localeCompare(fullName(b.user)));
  }, [chats, contacts]);
  const shownContacts = newChatContacts.filter((c) => matches(c.user));

  const unreadTotal = chats.reduce((n, c) => n + (c.unread_count || 0), 0);
  const onlineCount = contacts.filter((c) => c.online).length;

  if (authLoading || loading) {
    return (
      <div className='flex justify-center items-center h-64'>
        <LoadingSpinner size='large' />
      </div>
    );
  }

  return (
    <div className='chats-page'>
      <header className='chats-hero'>
        <h1 className='chats-title'>Messages</h1>
        <p className='chats-subtitle'>
          Chat privately with people you follow or who follow you.
        </p>
        <div className='chats-hero-stats'>
          <span className='chats-hero-chip'><Icon name='message' /> {chats.length} conversation{chats.length === 1 ? "" : "s"}</span>
          {unreadTotal > 0 && (
            <span className='chats-hero-chip chats-hero-chip--alert'>
              {unreadTotal} unread
            </span>
          )}
          <span className='chats-hero-chip'><Icon name='dot' /> {onlineCount} online</span>
        </div>
      </header>

      <div className='chats-search'>
        <span className='chats-search-icon' aria-hidden='true'><Icon name='search' /></span>
        <input
          type='search'
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder='Search people and conversations'
          aria-label='Search people and conversations'
        />
      </div>

      <div className='chats-layout'>
        {/* ---- Conversations ---- */}
        <section className='chats-card'>
          <h2 className='chats-section-title'>Conversations</h2>
          {shownChats.length === 0 ? (
            <div className='chats-empty'>
              {chats.length === 0
                ? "No conversations yet - start one from the list of people you can message."
                : "No conversations match your search."}
            </div>
          ) : (
            <ul className='chats-list'>
              {shownChats.map((chat) => {
                const unread = chat.unread_count > 0;
                return (
                  <li key={chat.other_user_id}>
                    <button
                      type='button'
                      onClick={() => router.push(`/chat/${chat.other_user_id}`)}
                      className={`chats-row ${unread ? "chats-row--unread" : ""}`}>
                      <span className='chats-avatar'>
                        <Avatar user={chat.other_user} size='lg' />
                        {chat.online && <span className='chats-online-dot' title='Online' />}
                      </span>
                      <span className='chats-row-main'>
                        <span className='chats-row-top'>
                          <span className='chats-row-name'>{fullName(chat.other_user) || "Unknown"}</span>
                          {chat.last_message_at && (
                            <span className='chats-row-time'>
                              {formatDistanceToNow(new Date(chat.last_message_at), { addSuffix: true })}
                            </span>
                          )}
                        </span>
                        <span className='chats-row-bottom'>
                          <span className='chats-row-preview'>
                            {chat.last_message || "No messages yet"}
                          </span>
                          {unread && <span className='chats-unread'>{chat.unread_count}</span>}
                        </span>
                      </span>
                    </button>
                  </li>
                );
              })}
            </ul>
          )}
        </section>

        {/* ---- Start a new chat ---- */}
        <aside className='chats-card chats-contacts'>
          <h2 className='chats-section-title'>Start a chat</h2>
          {shownContacts.length === 0 ? (
            <div className='chats-empty chats-empty--small'>
              {newChatContacts.length === 0
                ? "Follow people (or get followed) to message them."
                : "Nobody matches your search."}
            </div>
          ) : (
            <ul className='chats-contact-list'>
              {shownContacts.map(({ user: u, online }) => (
                <li key={u.id}>
                  <button
                    type='button'
                    className='chats-contact'
                    onClick={() => router.push(`/chat/${u.id}`)}>
                    <span className='chats-avatar'>
                      <Avatar user={u} size='md' />
                      {online && <span className='chats-online-dot' title='Online' />}
                    </span>
                    <span className='chats-contact-text'>
                      <span className='chats-row-name'>{fullName(u)}</span>
                      <span className='chats-contact-sub'>
                        {online ? "Online now" : u.nickname ? `@${u.nickname}` : "Offline"}
                      </span>
                    </span>
                    <span className='chats-contact-go' aria-hidden='true'><Icon name='edit' /></span>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </aside>
      </div>
    </div>
  );
}
