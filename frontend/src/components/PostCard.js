"use client";

import { useState, useEffect } from "react";
import Link from "next/link";
import { useAuth } from "@/context/AuthContext";
import { formatDistanceToNow } from "@/lib/formatDistanceToNow";
import { toast } from "@/context/ToastContext";
import { apiFetch } from "@/lib/api";
import { useWebSocket } from "@/context/WebSocketContext";
import PrivacyBadge from "@/components/PrivacyBadge";

export default function PostCard({
  post,
  onDelete,
  highlighted = false,
  autoOpenComments = false,
  highlightCommentId = null,
}) {
  const { user } = useAuth();
  const { subscribe } = useWebSocket();
  const [showComments, setShowComments] = useState(autoOpenComments);
  const [comments, setComments] = useState([]);
  const [commentContent, setCommentContent] = useState("");
  const [commentImage, setCommentImage] = useState(null);
  const [commentImagePreview, setCommentImagePreview] = useState(null);
  const [loadingComments, setLoadingComments] = useState(false);
  const [submittingComment, setSubmittingComment] = useState(false);

  // ✅ Like state
  const [liked, setLiked] = useState(post.is_liked || false);
  const [likeCount, setLikeCount] = useState(post.like_count || 0);

  // The parent list keeps like counts live (useLivePosts) - follow it.
  useEffect(() => {
    setLiked(post.is_liked || false);
    setLikeCount(post.like_count || 0);
  }, [post.is_liked, post.like_count]);

  // Someone (maybe this user in another tab) commented on this post -
  // show it right away in the open thread.
  useEffect(() => {
    return subscribe("comment_created", ({ post_id, comment }) => {
      if (post_id === post.id) addComment(comment);
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [post.id, subscribe]);

  const [saved, setSaved] = useState(post.is_saved || false);
  const [imageError, setImageError] = useState(false);
  const isOwnPost = user?.id === post.user_id;

  // Comments are oldest-first; the same comment can arrive both from our
  // own POST response and the live event, so skip ones already shown.
  const addComment = (comment) => {
    setComments((prev) =>
      prev.some((c) => c.id === comment.id) ? prev : [...prev, comment],
    );
  };

  const fetchComments = async () => {
    setLoadingComments(true);
    try {
      const { ok, data } = await apiFetch(`/post/comments?post_id=${post.id}`);
      setComments(ok && Array.isArray(data) ? data : []);
    } catch (error) {
      console.error("Error fetching comments:", error);
      setComments([]);
    } finally {
      setLoadingComments(false);
    }
  };

  const handleCommentImageChange = (e) => {
    const file = e.target.files[0];
    if (file) {
      const validTypes = ["image/jpeg", "image/png", "image/gif"];
      if (!validTypes.includes(file.type)) {
        toast.error("Please upload a valid image (JPEG, PNG, or GIF)");
        e.target.value = "";
        return;
      }
      if (file.size > 5 * 1024 * 1024) {
        toast.error("Image size must be less than 5MB");
        e.target.value = "";
        return;
      }
      setCommentImage(file);
      const reader = new FileReader();
      reader.onloadend = () => {
        setCommentImagePreview(reader.result);
      };
      reader.readAsDataURL(file);
    }
  };

  const removeCommentImage = () => {
    setCommentImage(null);
    setCommentImagePreview(null);
  };

  const handleCommentSubmit = async (e) => {
    e.preventDefault();
    if (!commentContent.trim() && !commentImage) {
      toast.error("Please write something or add an image");
      return;
    }

    setSubmittingComment(true);
    try {
      const formData = new FormData();
      formData.append("content", commentContent || "");
      formData.append("post_id", post.id);
      if (commentImage) {
        formData.append("image", commentImage);
      }

      const { ok, data } = await apiFetch("/post/comment", {
        method: "POST",
        body: formData,
      });

      if (ok) {
        addComment(data.comment);
        setCommentContent("");
        removeCommentImage();
        toast.success("Comment added!");
      } else {
        toast.error(data.error || "Failed to add comment");
      }
    } catch (error) {
      console.error("Error adding comment:", error);
      toast.error("Failed to add comment");
    } finally {
      setSubmittingComment(false);
    }
  };

  const toggleComments = () => {
    if (!showComments) {
      fetchComments();
    }
    setShowComments(!showComments);
  };

  useEffect(() => {
    if (autoOpenComments) {
      fetchComments();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    if (!highlightCommentId || comments.length === 0) return;
    const el = document.getElementById(`comment-${highlightCommentId}`);
    if (el) el.scrollIntoView({ behavior: "smooth", block: "center" });
  }, [comments, highlightCommentId]);

  // ✅ FIXED: Like handler with better error handling
  const handleLike = async () => {
    try {
      const { ok, status, data } = await apiFetch(`/post/like?id=${post.id}`, {
        method: "POST",
      });

      if (!ok) {
        throw new Error(data?.error || `Server responded with ${status}`);
      }

      setLiked(data.liked);
      setLikeCount(data.like_count || 0);
    } catch (error) {
      console.error("Error liking post:", error);
      toast.error("Failed to like post: " + error.message);
    }
  };

  const handleSave = async () => {
    try {
      const { ok, data } = await apiFetch(`/post/save?id=${post.id}`, {
        method: "POST",
      });

      if (ok) {
        setSaved(data.saved);
        toast.success(data.message);
      } else {
        toast.error("Failed to save/unsave post");
      }
    } catch (error) {
      console.error("Error saving post:", error);
      toast.error("Failed to save post");
    }
  };

  const handleDelete = async () => {
    if (!confirm("Are you sure you want to delete this post?")) return;

    try {
      const { ok, data } = await apiFetch(`/post/delete?id=${post.id}`, {
        method: "DELETE",
      });

      if (ok) {
        toast.success("Post deleted!");
        if (onDelete) onDelete(post.id);
      } else {
        toast.error(data.error || "Failed to delete post");
      }
    } catch (error) {
      console.error("Error deleting post:", error);
      toast.error("Failed to delete post");
    }
  };

  const formatDate = (dateString) => {
    if (!dateString) return "";
    try {
      const date = new Date(dateString);
      if (isNaN(date.getTime())) return "";
      return formatDistanceToNow(date, { addSuffix: true });
    } catch {
      return "";
    }
  };

  return (
    <div
      id={`post-${post.id}`}
      className={`bg-white rounded-lg shadow p-4 mb-4 transition-shadow ${
        highlighted ? "ring-2 ring-blue-400" : ""
      }`}>
      {/* Post Header */}
      <div className='flex items-start justify-between'>
        <div className='flex items-start space-x-3'>
          <Link href={`/profile/${post.user_id}`}>
            <div className='w-10 h-10 rounded-full bg-blue-500 flex items-center justify-center text-white font-bold hover:opacity-80'>
              {post.user?.first_name?.[0] || "U"}
            </div>
          </Link>
          <div>
            <p className='font-semibold'>
              <Link
                href={`/profile/${post.user_id}`}
                className='hover:underline'>
                {post.user?.first_name} {post.user?.last_name}
              </Link>
            </p>
            <p className='text-sm text-gray-500'>
              {formatDate(post.created_at)}
              <PrivacyBadge post={post} className='ml-2 bg-gray-100' />
            </p>
          </div>
        </div>
        {isOwnPost && (
          <button
            onClick={handleDelete}
            className='text-gray-400 hover:text-red-500'>
            ×
          </button>
        )}
      </div>

      {/* Post Content */}
      {post.content?.trim() && (
        <p className='mt-3 whitespace-pre-wrap break-words'>{post.content}</p>
      )}

      {/* Post Image */}
      {post.image_path && !imageError && (
        <div className='mt-3'>
          <img
            src={`http://localhost:8081${post.image_path}`}
            alt='Post image'
            className='rounded-lg max-h-96 w-full object-cover border'
            onError={() => setImageError(true)}
          />
        </div>
      )}

      {/* ✅ Post Actions */}
      <div className='flex items-center space-x-6 mt-3 pt-3 border-t'>
        {/* Like Button */}
        <button
          onClick={handleLike}
          className={`flex items-center space-x-1 text-sm transition ${
            liked ? "text-red-500" : "text-gray-500 hover:text-red-500"
          }`}>
          <span>{liked ? "❤️" : "🤍"}</span>
          <span className='font-medium'>{likeCount}</span>
        </button>

        {/* Comment Button */}
        <button
          onClick={toggleComments}
          className='flex items-center space-x-1 text-sm text-gray-500 hover:text-blue-500 transition'>
          <span>💬</span>
          <span>{Math.max(post.comment_count || 0, comments.length)}</span>
        </button>

        {/* Save Button */}
        <button
          onClick={handleSave}
          className={`flex items-center space-x-1 text-sm transition ${
            saved ? "text-blue-500" : "text-gray-500 hover:text-blue-500"
          }`}>
          <span>📌</span>
          <span>{saved ? "Saved" : "Save"}</span>
        </button>
      </div>

      {/* Comments Section */}
      {showComments && (
        <div className='mt-3 pt-3 border-t'>
          <form onSubmit={handleCommentSubmit} className='space-y-2'>
            <div className='flex space-x-2'>
              <input
                type='text'
                value={commentContent}
                onChange={(e) => setCommentContent(e.target.value)}
                placeholder='Write a comment...'
                className='flex-1 px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500'
                disabled={submittingComment}
              />
              <button
                type='submit'
                disabled={
                  (!commentContent.trim() && !commentImage) || submittingComment
                }
                className='px-4 py-2 bg-blue-500 text-white rounded-lg hover:bg-blue-600 transition disabled:opacity-50'>
                {submittingComment ? "..." : "Post"}
              </button>
            </div>

            <div className='flex items-center space-x-2'>
              <label className='cursor-pointer text-gray-500 hover:text-blue-500 text-sm'>
                <input
                  type='file'
                  accept='image/jpeg,image/png,image/gif'
                  onChange={handleCommentImageChange}
                  className='hidden'
                  disabled={submittingComment}
                />
                📷 Add Image
              </label>
              {commentImagePreview && (
                <div className='relative inline-block'>
                  <img
                    src={commentImagePreview}
                    alt='Comment preview'
                    className='h-12 w-12 rounded object-cover border'
                  />
                  <button
                    type='button'
                    onClick={removeCommentImage}
                    className='absolute -top-1 -right-1 bg-red-500 text-white rounded-full w-4 h-4 text-xs flex items-center justify-center'>
                    ×
                  </button>
                </div>
              )}
            </div>
          </form>

          <div className='mt-3 space-y-3 max-h-60 overflow-y-auto'>
            {loadingComments ? (
              <p className='text-center text-gray-400'>Loading comments...</p>
            ) : comments.length === 0 ? (
              <p className='text-center text-gray-400'>No comments yet</p>
            ) : (
              comments.map((comment) => (
                <div
                  key={comment.id}
                  id={`comment-${comment.id}`}
                  className={`flex items-start space-x-2 rounded-lg transition-colors ${
                    highlightCommentId === comment.id
                      ? "bg-blue-50 ring-1 ring-blue-300 p-2 -m-2"
                      : ""
                  }`}>
                  <Link href={`/profile/${comment.user_id}`}>
                    <div className='w-8 h-8 rounded-full bg-blue-500 flex items-center justify-center text-white text-sm font-bold hover:opacity-80'>
                      {comment.user?.first_name?.[0] || "U"}
                    </div>
                  </Link>
                  <div className='flex-1'>
                    <p className='text-sm font-semibold'>
                      <Link
                        href={`/profile/${comment.user_id}`}
                        className='hover:underline'>
                        {comment.user?.first_name} {comment.user?.last_name}
                      </Link>
                    </p>
                    {comment.content?.trim() && (
                      <p className='text-sm whitespace-pre-wrap break-words'>
                        {comment.content}
                      </p>
                    )}
                    {comment.image_path && (
                      <img
                        src={`http://localhost:8081${comment.image_path}`}
                        alt='Comment image'
                        className='mt-1 rounded max-h-32 object-cover'
                        onError={(e) => (e.target.style.display = "none")}
                      />
                    )}
                    <p className='text-xs text-gray-400'>
                      {formatDate(comment.created_at)}
                    </p>
                  </div>
                </div>
              ))
            )}
          </div>
        </div>
      )}
    </div>
  );
}
