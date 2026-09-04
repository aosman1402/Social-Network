"use client";

import { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import PostForm from "@/components/PostForm";
import PostCard from "@/components/PostCard";

export default function HomePage() {
  const router = useRouter();
  const [posts, setPosts] = useState([]);
  const [loading, setLoading] = useState(true);
  const [user, setUser] = useState(null);

  const fetchPosts = async () => {
    try {
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
    const checkAuth = async () => {
      try {
        const response = await fetch("http://localhost:8081/api/check-auth", {
          credentials: "include",
        });

        if (response.ok) {
          const data = await response.json();
          setUser(data);
          await fetchPosts();
        } else {
          router.push("/login");
        }
      } catch (error) {
        console.error("Auth check error:", error);
        router.push("/login");
      } finally {
        setLoading(false);
      }
    };

    checkAuth();
  }, [router]);

  useEffect(() => {
    async function loadPosts() {
      try {
        const res = await fetch("/api/posts");
        if (!res.ok) throw new Error(`Failed to fetch posts: ${res.status}`);
        const data = await res.json();
        setPosts(data);
      } catch (err) {
        // Log the error and update UI state to show an error message to the user
        console.error(err.message);
        setErrorState(true);
      }
    }
    loadPosts();
  }, []);

  const handlePostCreated = () => {
    fetchPosts();
  };

  const handlePostDeleted = (postId) => {
    setPosts(posts.filter((p) => p.id !== postId));
  };

  if (loading) {
    return (
      <div className='flex justify-center items-center h-64'>
        <div className='animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600'></div>
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
