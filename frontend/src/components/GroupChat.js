"use client";

import { useState, useEffect, useRef, useCallback } from "react";
import { apiFetch } from "@/lib/api";
import { toast } from "@/context/ToastContext";
import { useAuth } from "@/context/AuthContext";
import { useWebSocket } from "@/context/WebSocketContext";
import { formatDistanceToNow } from "@/lib/formatDistanceToNow";
import { getInitials, getAvatarSrc } from "@/utils/helpers";
import {
  mergeMessages,
  insertAtCursor,
  charCount,
  CHAT_EMOJIS,
  MAX_CHAT_MESSAGE_LENGTH,
} from "@/utils/chat";

// How long a "typing" indicator stays up without a fresh typing event, and
// how long after the last keystroke we tell the others we stopped.
const TYPING_DISPLAY_MS = 4000;
const TYPING_IDLE_MS = 2000;

// Live chat for one group, shown next to the group's posts. Messages are
// sent over the shared socket and the server delivers every message to all
// active members, the sender included - so nothing is added optimistically
// here: the server's copy is the one shown, exactly once (mergeMessages
// dedupes by id). onUnavailable is called when the server says the viewer
// isn't a member (e.g. they left in another tab), so the page can hide us.
export default function GroupChat({ groupId, members = [], onUnavailable }) {
  const gid = parseInt(groupId);
  const { user } = useAuth();
  const { isConnected, send, subscribe } = useWebSocket();

  const [messages, setMessages] = useState([]);
  const [loading, setLoading] = useState(true);
  const [draft, setDraft] = useState("");
  const [pickerOpen, setPickerOpen] = useState(false);
  const [typingUsers, setTypingUsers] = useState({}); // sender_id -> expiry timestamp

  const listRef = useRef(null);
  const inputRef = useRef(null);
  const pickerRef = useRef(null);
  const typingSentRef = useRef(false);
  const typingIdleTimerRef = useRef(null);

  // History - newest first from the server, merged into oldest -> newest.
  // Merging (rather than replacing) keeps any live message that arrived
  // while the request was in flight.
  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    apiFetch(`/messages/group-history?group_id=${gid}`)
      .then(({ ok, status, data }) => {
        if (cancelled) return;
        if (status === 403) {
          onUnavailable?.();
          return;
        }
        if (ok && Array.isArray(data)) {
          setMessages((prev) => mergeMessages(prev, data));
        }
      })
      .catch((err) => {
        console.error("Error loading group chat:", err);
        if (!cancelled) toast.error("Failed to load group chat");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [gid, onUnavailable]);

  // Catch up after a dropped connection: anything sent while we were
  // offline never arrived live, so re-read the history and merge it in
  // (mergeMessages skips what's already shown).
  const wasDisconnectedRef = useRef(false);
  useEffect(() => {
    if (!isConnected) {
      wasDisconnectedRef.current = true;
      return;
    }
    if (!wasDisconnectedRef.current) return;
    wasDisconnectedRef.current = false;
    apiFetch(`/messages/group-history?group_id=${gid}`)
      .then(({ ok, data }) => {
        if (ok && Array.isArray(data)) setMessages((prev) => mergeMessages(prev, data));
      })
      .catch((err) => console.error("Error catching up group chat:", err));
  }, [isConnected, gid]);

  // Live messages for this group only.
  useEffect(() => {
    return subscribe("chat", (msg) => {
      if (msg.group_id !== gid) return;
      setMessages((prev) => mergeMessages(prev, [msg]));
      // Their message arrived, so they've stopped typing.
      setTypingUsers((prev) => {
        if (!(msg.sender_id in prev)) return prev;
        const next = { ...prev };
        delete next[msg.sender_id];
        return next;
      });
    });
  }, [subscribe, gid]);

  useEffect(() => {
    return subscribe("chat_error", (err) => {
      if (err.group_id !== gid) return;
      toast.error(err.error || "Failed to send message");
      if (err.error === "You are not a member of this group") onUnavailable?.();
    });
  }, [subscribe, gid, onUnavailable]);

  // Typing indicators from other members, each expiring on its own if the
  // "stopped typing" event never comes (closed tab, dropped connection).
  useEffect(() => {
    return subscribe("typing", (evt) => {
      if (evt.group_id !== gid || evt.sender_id === user?.id) return;
      setTypingUsers((prev) => {
        const next = { ...prev };
        if (evt.is_typing) next[evt.sender_id] = Date.now() + TYPING_DISPLAY_MS;
        else delete next[evt.sender_id];
        return next;
      });
    });
  }, [subscribe, gid, user]);

  useEffect(() => {
    if (Object.keys(typingUsers).length === 0) return;
    const timer = setInterval(() => {
      const now = Date.now();
      setTypingUsers((prev) => {
        const next = Object.fromEntries(
          Object.entries(prev).filter(([, expiry]) => expiry > now),
        );
        return Object.keys(next).length === Object.keys(prev).length ? prev : next;
      });
    }, 1000);
    return () => clearInterval(timer);
  }, [typingUsers]);

  // Keep the newest message in view - scrolling the list itself, not the
  // page, so the posts column doesn't jump.
  useEffect(() => {
    const list = listRef.current;
    if (list) list.scrollTop = list.scrollHeight;
  }, [messages, loading]);

  // Close the emoji picker on an outside click.
  useEffect(() => {
    if (!pickerOpen) return;
    const handleClick = (e) => {
      if (pickerRef.current && !pickerRef.current.contains(e.target)) {
        setPickerOpen(false);
      }
    };
    document.addEventListener("mousedown", handleClick);
    return () => document.removeEventListener("mousedown", handleClick);
  }, [pickerOpen]);

  const sendTyping = useCallback(
    (isTyping) => {
      if (typingSentRef.current === isTyping) return;
      typingSentRef.current = isTyping;
      send({ type: "typing", data: { group_id: gid, is_typing: isTyping } });
    },
    [send, gid],
  );

  // Tell the others we stopped typing when we leave the page.
  useEffect(() => {
    return () => {
      clearTimeout(typingIdleTimerRef.current);
      if (typingSentRef.current) {
        send({ type: "typing", data: { group_id: gid, is_typing: false } });
      }
    };
  }, [send, gid]);

  const handleDraftChange = (value) => {
    setDraft(value);
    clearTimeout(typingIdleTimerRef.current);
    if (value.trim()) {
      sendTyping(true);
      typingIdleTimerRef.current = setTimeout(() => sendTyping(false), TYPING_IDLE_MS);
    } else {
      sendTyping(false);
    }
  };

  const insertEmoji = (emoji) => {
    const input = inputRef.current;
    const { value, cursor } = insertAtCursor(
      draft,
      emoji,
      input?.selectionStart,
      input?.selectionEnd,
    );
    if (charCount(value) > MAX_CHAT_MESSAGE_LENGTH) return;
    handleDraftChange(value);
    // Put the cursor back after the inserted emoji once React re-renders.
    requestAnimationFrame(() => {
      input?.focus();
      input?.setSelectionRange(cursor, cursor);
    });
  };

  const sendMessage = (e) => {
    e.preventDefault();
    const content = draft.trim();
    if (!content || !isConnected) return;
    if (charCount(content) > MAX_CHAT_MESSAGE_LENGTH) {
      toast.error(`Messages can be at most ${MAX_CHAT_MESSAGE_LENGTH} characters`);
      return;
    }
    if (!send({ type: "chat", data: { group_id: gid, content } })) {
      toast.error("Not connected - try again in a moment");
      return;
    }
    setDraft("");
    setPickerOpen(false);
    clearTimeout(typingIdleTimerRef.current);
    sendTyping(false);
  };

  const memberName = (id) => {
    const m = members.find((mem) => mem.user?.id === id);
    return m ? m.user.first_name : "Someone";
  };
  const typingNames = Object.keys(typingUsers).map((id) => memberName(parseInt(id)));
  const remaining = MAX_CHAT_MESSAGE_LENGTH - charCount(draft);

  return (
    <section className='group-chat' aria-label='Group chat'>
      <header className='group-chat-header'>
        <h3 className='group-section-title group-chat-title'>Group chat</h3>
        <span
          className={`group-chat-status ${isConnected ? "group-chat-status--online" : ""}`}>
          {isConnected ? "Live" : "Reconnecting..."}
        </span>
      </header>

      <div className='group-chat-messages' ref={listRef}>
        {loading ? (
          <p className='group-empty'>Loading messages...</p>
        ) : messages.length === 0 ? (
          <p className='group-empty group-chat-empty'>
            No messages yet - say hello 👋
          </p>
        ) : (
          messages.map((msg) => {
            const own = msg.sender_id === user?.id;
            const sender = msg.sender || {};
            const avatarSrc = getAvatarSrc(sender.avatar_path);
            return (
              <div
                key={msg.id}
                className={`group-chat-row ${own ? "group-chat-row--own" : ""}`}>
                {!own && (
                  <div className='group-chat-avatar' aria-hidden='true'>
                    {avatarSrc ? (
                      <img src={avatarSrc} alt='' />
                    ) : (
                      getInitials(sender.first_name, sender.last_name)
                    )}
                  </div>
                )}
                <div className='group-chat-bubble'>
                  {!own && (
                    <p className='group-chat-sender'>
                      {`${sender.first_name || ""} ${sender.last_name || ""}`.trim() ||
                        "Unknown"}
                    </p>
                  )}
                  <p className='group-chat-text'>{msg.content}</p>
                  <p className='group-chat-time'>
                    {formatDistanceToNow(new Date(msg.created_at), { addSuffix: true })}
                  </p>
                </div>
              </div>
            );
          })
        )}
      </div>

      <p className='group-chat-typing' aria-live='polite'>
        {typingNames.length === 1 && `${typingNames[0]} is typing...`}
        {typingNames.length === 2 && `${typingNames[0]} and ${typingNames[1]} are typing...`}
        {typingNames.length > 2 && "Several people are typing..."}
      </p>

      <form className='group-chat-composer' onSubmit={sendMessage}>
        <div className='group-chat-emoji' ref={pickerRef}>
          <button
            type='button'
            className='group-chat-emoji-btn'
            onClick={() => setPickerOpen((v) => !v)}
            aria-label='Insert emoji'
            aria-expanded={pickerOpen}>
            😊
          </button>
          {pickerOpen && (
            <div className='group-chat-emoji-grid' role='listbox' aria-label='Emoji'>
              {CHAT_EMOJIS.map((emoji) => (
                <button
                  key={emoji}
                  type='button'
                  className='group-chat-emoji-option'
                  onClick={() => insertEmoji(emoji)}>
                  {emoji}
                </button>
              ))}
            </div>
          )}
        </div>
        <input
          ref={inputRef}
          type='text'
          className='group-chat-input'
          value={draft}
          onChange={(e) => handleDraftChange(e.target.value)}
          placeholder={isConnected ? "Message the group..." : "Reconnecting..."}
          disabled={!isConnected}
          aria-label='Message'
        />
        <button
          type='submit'
          className='group-chat-send'
          disabled={!isConnected || !draft.trim() || remaining < 0}>
          Send
        </button>
      </form>
      {remaining < 100 && (
        <p className={`group-chat-count ${remaining < 0 ? "group-chat-count--over" : ""}`}>
          {remaining} characters left
        </p>
      )}
    </section>
  );
}
