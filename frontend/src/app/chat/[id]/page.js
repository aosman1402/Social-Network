"use client";

import { useState, useEffect, useRef } from "react";
import { useParams, useRouter } from "next/navigation";
import { formatDistanceToNow } from "@/lib/formatDistanceToNow";
import { toast } from "@/context/ToastContext";
import { apiFetch } from "@/lib/api";
import LoadingSpinner from "@/components/LoadingSpinner";
import Avatar from "@/components/Avatar";
import Icon from "@/components/Icon";
import { useAuth } from "@/context/AuthContext";
import { useWebSocket } from "@/context/WebSocketContext";
import {
  mergeMessages,
  confirmPendingMessage,
  dropPendingMessage,
  insertAtCursor,
  CHAT_EMOJIS,
} from "@/utils/chat";

// "Today" / "Yesterday" / "Mon 3 Mar 2026" - for the separators between days.
const dayLabel = (date) => {
  const d = new Date(date);
  const today = new Date();
  const yesterday = new Date();
  yesterday.setDate(today.getDate() - 1);
  if (d.toDateString() === today.toDateString()) return "Today";
  if (d.toDateString() === yesterday.toDateString()) return "Yesterday";
  return d.toLocaleDateString(undefined, { weekday: "short", day: "numeric", month: "short", year: "numeric" });
};

const clockTime = (date) =>
  new Date(date).toLocaleTimeString(undefined, { hour: "2-digit", minute: "2-digit" });

// How long "typing…" stays up without a fresh typing event, and how long
// after our last keystroke we tell the other person we stopped.
const TYPING_DISPLAY_MS = 4000;
const TYPING_IDLE_MS = 2000;

// Temporary ids for the sender's optimistic messages, unique per tab.
let nextClientId = 1;
const newClientId = () => `tmp-${Date.now()}-${nextClientId++}`;

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
  // Whether the OTHER person is online (null = unknown). Not our own
  // connection - that's isConnected.
  const [otherOnline, setOtherOnline] = useState(null);
  const messagesEndRef = useRef(null);
  const [otherTyping, setOtherTyping] = useState(false);
  const [pickerOpen, setPickerOpen] = useState(false);
  const inputRef = useRef(null);
  const pickerRef = useRef(null);
  const typingSentRef = useRef(false);
  const typingIdleTimerRef = useRef(null);
  const otherTypingTimerRef = useRef(null);

  // ---- Typing indicator (the server relays it only between people who
  // may message each other) --------------------------------------------
  const sendTyping = (isTyping) => {
    if (typingSentRef.current === isTyping) return;
    typingSentRef.current = isTyping;
    send({ type: "typing", data: { recipient_id: parseInt(userId), is_typing: isTyping } });
  };

  useEffect(() => {
    const otherUserId = parseInt(userId);
    return subscribe("typing", (evt) => {
      if (evt.group_id || evt.sender_id !== otherUserId) return;
      clearTimeout(otherTypingTimerRef.current);
      setOtherTyping(!!evt.is_typing);
      if (evt.is_typing) {
        // Clears itself if the "stopped" event never comes (closed tab).
        otherTypingTimerRef.current = setTimeout(() => setOtherTyping(false), TYPING_DISPLAY_MS);
      }
    });
  }, [subscribe, userId]);

  // Their message arrived, so they've stopped typing; and tell them we
  // stopped when we leave the conversation.
  useEffect(() => {
    const otherUserId = parseInt(userId);
    const unsub = subscribe("chat", (msg) => {
      if (msg.sender_id === otherUserId) setOtherTyping(false);
    });
    return () => {
      unsub();
      clearTimeout(typingIdleTimerRef.current);
      clearTimeout(otherTypingTimerRef.current);
      if (typingSentRef.current) {
        send({ type: "typing", data: { recipient_id: otherUserId, is_typing: false } });
        typingSentRef.current = false;
      }
    };
  }, [subscribe, send, userId]);

  const handleDraftChange = (value) => {
    setNewMessage(value);
    clearTimeout(typingIdleTimerRef.current);
    if (value.trim()) {
      sendTyping(true);
      typingIdleTimerRef.current = setTimeout(() => sendTyping(false), TYPING_IDLE_MS);
    } else {
      sendTyping(false);
    }
  };

  // ---- Emoji picker ------------------------------------------------------
  useEffect(() => {
    if (!pickerOpen) return;
    const handleClick = (e) => {
      if (pickerRef.current && !pickerRef.current.contains(e.target)) setPickerOpen(false);
    };
    document.addEventListener("mousedown", handleClick);
    return () => document.removeEventListener("mousedown", handleClick);
  }, [pickerOpen]);

  const insertEmoji = (emoji) => {
    const input = inputRef.current;
    const { value, cursor } = insertAtCursor(newMessage, emoji, input?.selectionStart, input?.selectionEnd);
    handleDraftChange(value);
    requestAnimationFrame(() => {
      input?.focus();
      input?.setSelectionRange(cursor, cursor);
    });
  };

  // Load the other user's profile and this conversation's history.
  useEffect(() => {
    // Start empty for each conversation - the page is reused when moving
    // between chats, and history is merged into (not swapped for) the list.
    setMessages([]);
    setBlocked(false);
    setLoading(true);

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

        // History arrives newest first; show it oldest -> newest, the same
        // order live messages are added in. Merging also keeps any message
        // that arrived live while the request was in flight.
        setMessages((prev) =>
          mergeMessages(prev, msgOk && Array.isArray(msgData) ? msgData : []),
        );
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

  // Catch up after a dropped connection - messages sent while we were
  // offline never arrived live. Merged, so nothing already shown repeats.
  const wasDisconnectedRef = useRef(false);
  useEffect(() => {
    if (!isConnected) {
      wasDisconnectedRef.current = true;
      return;
    }
    if (!wasDisconnectedRef.current) return;
    wasDisconnectedRef.current = false;
    apiFetch(`/messages/history?user_id=${userId}`)
      .then(({ ok, data }) => {
        if (ok && Array.isArray(data)) setMessages((prev) => mergeMessages(prev, data));
      })
      .catch((error) => console.error("Error catching up chat:", error));
  }, [isConnected, userId]);

  // The other person's online status: read it whenever our own connection
  // is (re)established - a change while we were offline was never pushed to
  // us - then keep it current from live "presence" events.
  useEffect(() => {
    setOtherOnline(null);
    if (!isConnected || blocked) return;
    let cancelled = false;
    apiFetch(`/user/presence?id=${userId}`)
      .then(({ ok, data }) => {
        if (!cancelled) setOtherOnline(ok ? !!data.online : null);
      })
      .catch(() => {});
    return () => {
      cancelled = true;
    };
  }, [isConnected, blocked, userId]);

  useEffect(() => {
    const otherUserId = parseInt(userId);
    return subscribe("presence", (p) => {
      if (p.user_id === otherUserId) setOtherOnline(!!p.online);
    });
  }, [subscribe, userId]);

  // Live-append incoming messages that belong to this conversation. The
  // shared socket delivers every "chat" event addressed to this user, not
  // just ones from the person currently open here, so it has to be
  // filtered by conversation partner.
  useEffect(() => {
    const otherUserId = parseInt(userId);
    return subscribe("chat", (msg) => {
      if (msg.group_id) return; // group chat, not this DM
      if (msg.sender_id !== otherUserId && msg.recipient_id !== otherUserId) {
        return;
      }
      // Deduped by server id, so a message can never be shown twice.
      setMessages((prev) => mergeMessages(prev, [msg]));
    });
  }, [subscribe, userId]);

  // The server confirmed one of our WebSocket sends: swap the optimistic
  // copy's temporary id for the real one, so a later history reload or a
  // repeated event can't produce a second copy.
  useEffect(() => {
    const otherUserId = parseInt(userId);
    return subscribe("ack", (ack) => {
      if (ack.recipient_id !== otherUserId || !ack.client_id) return;
      setMessages((prev) => confirmPendingMessage(prev, ack.client_id, ack));
    });
  }, [subscribe, userId]);

  // A WebSocket "chat" send is optimistic on the frontend (see sendMessage
  // below) - if the server rejects it (e.g. no follow relationship), the
  // sender otherwise never finds out and the message stays shown as if it
  // went through. Roll back that optimistic entry and surface why.
  useEffect(() => {
    const otherUserId = parseInt(userId);
    return subscribe("chat_error", (err) => {
      if (err.recipient_id !== otherUserId) return;
      setMessages((prev) =>
        err.client_id
          ? dropPendingMessage(prev, err.client_id)
          : prev.filter((m) => !m.pending),
      );
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
    setPickerOpen(false);
    clearTimeout(typingIdleTimerRef.current);
    sendTyping(false);

    const clientId = newClientId();
    const optimisticMsg = {
      id: clientId,
      client_id: clientId,
      sender_id: user.id,
      content,
      created_at: new Date().toISOString(),
      sender: { first_name: "You", last_name: "" },
      // Cleared by the server's "ack" (which swaps in the real id), or the
      // message is removed by a chat_error - both matched by client_id,
      // since the WS send below doesn't wait for a response before showing it.
      pending: true,
    };

    // Try WebSocket first
    if (
      send({
        type: "chat",
        data: { recipient_id: parseInt(userId), content, client_id: clientId },
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

      setMessages((prev) =>
        mergeMessages(prev, [
          {
            ...optimisticMsg,
            id: data.message_id || optimisticMsg.id,
            pending: !data.message_id,
          },
        ]),
      );
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
      <div className='dm-page'>
        <div className='dm-state-card'>
          <h2 className='dm-state-title'>User not found</h2>
          <button onClick={() => router.push("/chat")} className='dm-btn dm-btn--primary'>
            Back to Messages
          </button>
        </div>
      </div>
    );
  }

  const otherName = `${otherUser.first_name || ""} ${otherUser.last_name || ""}`.trim();

  // Neither user follows the other - direct messages require a follow
  // relationship in either direction, so there's no conversation to show.
  if (blocked) {
    return (
      <div className='dm-page'>
        <div className='dm-state-card'>
          <Avatar user={otherUser} size='xl' />
          <h2 className='dm-state-title'>{otherName}</h2>
          <p className='dm-state-text'>
            <Icon name='lock' /> You can&apos;t message {otherUser.first_name} yet. You can only send
            direct messages to people who follow you or that you follow.
          </p>
          <div className='dm-state-actions'>
            <button onClick={() => router.push(`/profile/${userId}`)} className='dm-btn dm-btn--primary'>
              View Profile
            </button>
            <button onClick={() => router.push("/chat")} className='dm-btn dm-btn--secondary'>
              Back to Messages
            </button>
          </div>
        </div>
      </div>
    );
  }

  // Status line under their name: typing beats online/offline; while our
  // own connection is down we can't know either.
  const status = !isConnected
    ? { text: "Reconnecting…", tone: "muted" }
    : otherTyping
      ? { text: "typing…", tone: "typing" }
      : otherOnline === null
        ? null
        : otherOnline
          ? { text: "Online", tone: "online" }
          : { text: "Offline", tone: "muted" };

  return (
    <div className='dm-page'>
      <div className='dm-card'>
        {/* Header */}
        <div className='dm-header'>
          <button
            type='button'
            className='dm-header-person'
            onClick={() => router.push(`/profile/${userId}`)}
            title='View profile'>
            <span className='chats-avatar'>
              <Avatar user={otherUser} size='lg' />
              {isConnected && otherOnline && <span className='chats-online-dot' />}
            </span>
            <span className='dm-header-text'>
              <span className='dm-header-name'>{otherName}</span>
              {status && (
                <span className={`dm-status dm-status--${status.tone}`}>{status.text}</span>
              )}
            </span>
          </button>
        </div>

        {/* Messages */}
        <div className='dm-messages'>
          {messages.length === 0 ? (
            <div className='dm-empty'>
              <Avatar user={otherUser} size='xl' />
              <p className='dm-empty-title'>This is the start of your chat with {otherUser.first_name}</p>
              <p className='dm-empty-sub'>Say hello!</p>
            </div>
          ) : (
            messages.map((msg, index) => {
              const isTheirs = msg.sender_id === parseInt(userId);
              const prev = messages[index - 1];
              const newDay =
                !prev || new Date(prev.created_at).toDateString() !== new Date(msg.created_at).toDateString();
              // Consecutive messages from the same person are grouped.
              const grouped = !newDay && prev && prev.sender_id === msg.sender_id;
              return (
                <div key={msg.client_id || msg.id || index}>
                  {newDay && (
                    <div className='dm-day'>
                      <span>{dayLabel(msg.created_at)}</span>
                    </div>
                  )}
                  <div className={`dm-row ${isTheirs ? "dm-row--theirs" : "dm-row--mine"} ${grouped ? "dm-row--grouped" : ""}`}>
                    <div
                      className={`dm-bubble ${isTheirs ? "dm-bubble--theirs" : "dm-bubble--mine"} ${msg.pending ? "dm-bubble--pending" : ""}`}
                      title={formatDistanceToNow(new Date(msg.created_at), { addSuffix: true })}>
                      <p className='dm-bubble-text'>{msg.content}</p>
                      <span className='dm-bubble-time'>
                        {clockTime(msg.created_at)}
                        {!isTheirs && (msg.pending ? " · sending" : " ✓")}
                      </span>
                    </div>
                  </div>
                </div>
              );
            })
          )}
          <div ref={messagesEndRef} />
        </div>

        {/* Composer */}
        <form onSubmit={sendMessage} className='dm-composer'>
          <div className='dm-emoji' ref={pickerRef}>
            <button
              type='button'
              className='dm-emoji-btn'
              onClick={() => setPickerOpen((v) => !v)}
              aria-label='Insert emoji'
              aria-expanded={pickerOpen}>
              <Icon name='smile' />
            </button>
            {pickerOpen && (
              <div className='dm-emoji-grid' role='listbox' aria-label='Emoji'>
                {CHAT_EMOJIS.map((emoji) => (
                  <button key={emoji} type='button' className='dm-emoji-option' onClick={() => insertEmoji(emoji)}>
                    {emoji}
                  </button>
                ))}
              </div>
            )}
          </div>
          <input
            ref={inputRef}
            type='text'
            value={newMessage}
            onChange={(e) => handleDraftChange(e.target.value)}
            placeholder={isConnected ? `Message ${otherUser.first_name}…` : "Connecting…"}
            className='dm-input rounded-full'
            disabled={!isConnected && !sending}
            aria-label='Message'
          />
          <button
            type='submit'
            disabled={!newMessage.trim() || sending}
            className='dm-send'
            aria-label='Send'>
            {sending ? "…" : <Icon name='send' />}
          </button>
        </form>
      </div>
    </div>
  );
}
