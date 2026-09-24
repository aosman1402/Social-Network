"use client";

import { useEffect, useRef } from "react";
import { useAuth } from "@/context/AuthContext";
import { useWebSocket } from "@/context/WebSocketContext";

// Keeps a list of posts (held by the caller in setPosts state) live: new
// posts appear, deleted ones disappear, and like/comment counts update
// the moment anyone acts on them - no refresh needed. The server only
// sends these events to users allowed to see the post in question.
//
// accept(post) decides whether a newly created post belongs in this
// particular list (e.g. only this group's posts on a group page); pass
// enabled=false to ignore events entirely (e.g. while a list isn't shown).
export function useLivePosts(setPosts, { accept = () => true, enabled = true } = {}) {
  const { user } = useAuth();
  const { subscribe } = useWebSocket();
  const acceptRef = useRef(accept);
  acceptRef.current = accept;

  useEffect(() => {
    if (!enabled) return;

    const updatePost = (postId, update) =>
      setPosts((prev) => prev.map((p) => (p.id === postId ? { ...p, ...update(p) } : p)));

    const unsubs = [
      subscribe("post_created", (post) => {
        if (!acceptRef.current(post)) return;
        setPosts((prev) => (prev.some((p) => p.id === post.id) ? prev : [post, ...prev]));
      }),
      subscribe("post_deleted", ({ post_id }) => {
        setPosts((prev) => prev.filter((p) => p.id !== post_id));
      }),
      subscribe("comment_created", ({ post_id, comment_count }) => {
        updatePost(post_id, () => ({ comment_count }));
      }),
      subscribe("post_liked", ({ post_id, user_id, liked, like_count }) => {
        updatePost(post_id, (p) => ({
          like_count,
          is_liked: user_id === user?.id ? liked : p.is_liked,
        }));
      }),
    ];
    return () => unsubs.forEach((unsub) => unsub());
  }, [enabled, setPosts, subscribe, user?.id]);
}
