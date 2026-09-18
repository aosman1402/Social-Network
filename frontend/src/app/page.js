"use client";

import { useState, useEffect } from "react";
import PostForm from "@/components/PostForm";
import PostCard from "@/components/PostCard";
import LoadingSpinner from "@/components/LoadingSpinner";
import { useRequireAuth } from "@/hooks/useRequireAuth";

export default function HomePage() {
  const { user, loading } = useRequireAuth();
  const [posts, setPosts] = useState([]);

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

  const handlePostCreated = () => {
    fetchPosts();
  };

  const handlePostDeleted = (postId) => {
    setPosts(posts.filter((p) => p.id !== postId));
  };

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
          posts.map((post) => (
            <PostCard
              key={post.id}
              post={post}
              onUpdate={fetchPosts}
              onDelete={handlePostDeleted}
            />
          ))
        )}
      </div>
    </div>
  );
}
