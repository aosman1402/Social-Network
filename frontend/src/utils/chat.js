/**
 * Merge chat messages into an existing list, keeping each message exactly
 * once (by server id) in chronological order. Used by the group chat,
 * where the server sends a message back to its own sender too - so the same
 * message can arrive both from the history fetch and live over the socket,
 * or twice if two sockets briefly overlap on reconnect.
 *
 * @param {Array} existing - Messages already shown
 * @param {Array} incoming - Messages to add (any order)
 * @returns {Array} A new array, deduplicated and sorted oldest -> newest
 */
export const mergeMessages = (existing, incoming) => {
  const byId = new Map();
  for (const msg of existing) byId.set(msg.id, msg);
  for (const msg of incoming) {
    if (msg && msg.id != null && !byId.has(msg.id)) byId.set(msg.id, msg);
  }
  return [...byId.values()].sort((a, b) => {
    const diff = new Date(a.created_at) - new Date(b.created_at);
    if (diff !== 0) return diff;
    // Temporary (string) ids of unconfirmed messages have no numeric order.
    return typeof a.id === "number" && typeof b.id === "number" ? a.id - b.id : 0;
  });
};

/**
 * Swap a sender's optimistic message (identified by the temporary
 * clientId it was sent with) for the server's confirmed version, using the
 * "ack" the server sends back. If the real message is somehow already in
 * the list, the optimistic copy is dropped instead, so it can never show
 * twice.
 */
export const confirmPendingMessage = (messages, clientId, { message_id, created_at }) => {
  const alreadyThere = messages.some((m) => m.id === message_id);
  return messages.flatMap((m) => {
    if (m.client_id !== clientId || !m.pending) return [m];
    if (alreadyThere) return [];
    return [{ ...m, id: message_id, created_at: created_at || m.created_at, pending: false }];
  });
};

/** Remove one optimistic message whose send failed. */
export const dropPendingMessage = (messages, clientId) =>
  messages.filter((m) => !(m.pending && m.client_id === clientId));

/** Max chat message length, in characters - matches the backend's
 * MaxChatMessageLength (counted in code points, so an emoji is one). */
export const MAX_CHAT_MESSAGE_LENGTH = 1000;

/** Length of text in code points rather than UTF-16 units. */
export const charCount = (text) => Array.from(text).length;

/**
 * Insert text into value at the given selection, returning the new value
 * and where the cursor should go - used by the emoji picker to insert at
 * the cursor instead of always appending.
 */
export const insertAtCursor = (value, text, start, end = start) => {
  const from = start ?? value.length;
  const to = end ?? from;
  return {
    value: value.slice(0, from) + text + value.slice(to),
    cursor: from + text.length,
  };
};

/** A small, dependency-free set of emoji for the chat picker. */
export const CHAT_EMOJIS = [
  "😀", "😂", "🤣", "😊", "😍", "😘", "😎", "🤔",
  "😅", "😢", "😭", "😡", "😮", "🥳", "😴", "🙄",
  "👍", "👎", "👏", "🙏", "💪", "👋", "🤝", "✌️",
  "❤️", "🔥", "🎉", "✨", "💯", "✅",
];
