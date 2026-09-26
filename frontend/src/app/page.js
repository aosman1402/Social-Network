"use client";

import { useState, useEffect, useRef, Suspense } from "react";
import { useSearchParams } from "next/navigation";
import PostForm from "@/components/PostForm";
import PostCard from "@/components/PostCard";
import LoadingSpinner from "@/components/LoadingSpinner";
import { useRequireAuth } from "@/hooks/useRequireAuth";
import { useLivePosts } from "@/hooks/useLivePosts";
import { apiFetch } from "@/lib/api";

export default function HomePage() {
  return (
    <Suspense
      fallback={
        <div className='flex justify-center items-center h-64'>
          <LoadingSpinner size='large' />
        </div>
      }>
      <HomeFeed />
    </Suspense>
  );
}

function HomeFeed() {
  const { user, loading } = useRequireAuth();
  const searchParams = useSearchParams();
  const [posts, setPosts] = useState([]);

  // A notification (like/comment) can deep-link here with ?post=<id> and,
  // for a comment notification, &comment=<id> - the targeted post gets
  // scrolled to and briefly highlighted, with its comments auto-expanded
  // and the specific comment highlighted too.
  const targetPostId = searchParams.get("post");
  const targetCommentId = searchParams.get("comment");
  const [highlightedPostId, setHighlightedPostId] = useState(null);
  const handledTargetRef = useRef(null);

  // The feed shows every post the viewer can see, so any new one belongs.
  useLivePosts(setPosts);

  const fetchPosts = async () => {
    try {
      // Goes through the Next.js rewrite proxy (next.config.js) rather
      // than the backend directly - see the note in next.config.js on why
      // this one call is different from the rest of the app.
      const response = await fetch("/api/posts");
      if (!response.ok) {
        console.error("Failed to fetch posts:", response.status);
        setPosts([]);
        return;
      }

      const data = await response.json();
      setPosts(Array.isArray(data) ? data : []);
    } catch (error) {
      console.error("Error fetching posts:", error);
      setPosts([]);
    }
  };

  useEffect(() => {
    if (user) fetchPosts();
  }, [user]);

  // Make sure the deep-linked post is actually loaded (it may be older than
  // the feed page, or the notified user may not follow its author but is
  // still allowed to view it, e.g. as a named allowed_user on a private
  // post), then scroll to it and highlight it briefly.
  useEffect(() => {
    if (!targetPostId || posts.length === 0) return;
    if (handledTargetRef.current === targetPostId) return;

    const targetId = parseInt(targetPostId, 10);
    const loadedPost = posts.find((p) => p.id === targetId);

    if (loadedPost) {
      handledTargetRef.current = targetPostId;
      setHighlightedPostId(targetId);
      const el = document.getElementById(`post-${targetId}`);
      if (el) el.scrollIntoView({ behavior: "smooth", block: "start" });
      const timeout = setTimeout(() => setHighlightedPostId(null), 3000);
      return () => clearTimeout(timeout);
    }

    let cancelled = false;
    (async () => {
      const { ok, data } = await apiFetch(`/post/get?id=${targetId}`);
      if (!ok || cancelled) return;
      setPosts((prev) =>
        prev.some((p) => p.id === data.id) ? prev : [data, ...prev],
      );
    })();
    return () => {
      cancelled = true;
    };
  }, [targetPostId, posts]);

  const handlePostCreated = () => {
    fetchPosts();
  };

  const handlePostDeleted = (postId) => {
    setPosts(posts.filter((p) => p.id !== postId));
  };

  // Pinned posts (the viewer's own pins) stay at the top of the feed.
  const handlePinChange = (postId, pinned) => {
    setPosts((prev) => prev.map((p) => (p.id === postId ? { ...p, is_saved: pinned } : p)));
  };
  const pinnedPosts = posts.filter((p) => p.is_saved);
  const otherPosts = posts.filter((p) => !p.is_saved);

  const renderPost = (post) => (
    <PostCard
      key={post.id}
      post={post}
      onDelete={handlePostDeleted}
      showPin
      onPinChange={handlePinChange}
      highlighted={highlightedPostId === post.id}
      autoOpenComments={
        !!targetCommentId && targetPostId
          ? post.id === parseInt(targetPostId, 10)
          : false
      }
      highlightCommentId={
        targetCommentId && post.id === parseInt(targetPostId, 10)
          ? parseInt(targetCommentId, 10)
          : null
      }
    />
  );

  if (loading) {
    return (
      <div className='flex justify-center items-center h-64'>
        <LoadingSpinner size='large' />
      </div>
    );
  }

  if (!user) {
    return null;
  }

  return (
    <div className='max-w-2xl mx-auto py-4 px-4'>
      <PostForm onPostCreated={handlePostCreated} />

      <div className='space-y-4'>
        {posts.length === 0 ? (
          <div className='bg-white rounded-lg shadow p-8 text-center'>
            <p className='text-gray-500'>No posts yet</p>
            <p className='text-sm text-gray-400 mt-2'>Be the first to post!</p>
          </div>
        ) : (
          <>
            {pinnedPosts.length > 0 && (
              <h2 className='feed-section-title'>📌 Pinned</h2>
            )}
            {pinnedPosts.map(renderPost)}
            {pinnedPosts.length > 0 && otherPosts.length > 0 && (
              <h2 className='feed-section-title'>Latest posts</h2>
            )}
            {otherPosts.map(renderPost)}
          </>
        )}
      </div>
    </div>
  );
}
