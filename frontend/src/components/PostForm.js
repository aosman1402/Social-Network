"use client";

import { useState } from "react";
import { useAuth } from "@/context/AuthContext";
import { toast } from "@/context/ToastContext";
import { apiFetch } from "@/lib/api";

export default function PostForm({ onPostCreated }) {
  const { user } = useAuth();
  const [content, setContent] = useState("");
  const [privacy, setPrivacy] = useState("public");
  const [image, setImage] = useState(null);
  const [imagePreview, setImagePreview] = useState(null);
  const [loading, setLoading] = useState(false);

  // "Private" posts go to a hand-picked subset of the author's followers
  // (only they can see it besides the author) rather than every follower.
  const [followers, setFollowers] = useState([]);
  const [loadingFollowers, setLoadingFollowers] = useState(false);
  const [followersLoaded, setFollowersLoaded] = useState(false);
  const [selectedViewerIds, setSelectedViewerIds] = useState([]);

  const handlePrivacyChange = (value) => {
    setPrivacy(value);
    if (value === "private" && !followersLoaded && user?.id) {
      setLoadingFollowers(true);
      apiFetch(`/user/followers?id=${user.id}`)
        .then(({ ok, data }) => (ok ? data : []))
        .then((data) => setFollowers(Array.isArray(data) ? data : []))
        .catch((error) => {
          console.error("Error loading followers:", error);
          toast.error("Failed to load your followers");
        })
        .finally(() => {
          setLoadingFollowers(false);
          setFollowersLoaded(true);
        });
    }
  };

  const toggleViewer = (id) => {
    setSelectedViewerIds((prev) =>
      prev.includes(id) ? prev.filter((v) => v !== id) : [...prev, id],
    );
  };

  const handleImageChange = (e) => {
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

      setImage(file);
      const reader = new FileReader();
      reader.onloadend = () => {
        setImagePreview(reader.result);
      };
      reader.readAsDataURL(file);
    }
  };

  const removeImage = () => {
    setImage(null);
    setImagePreview(null);
    const fileInput = document.getElementById("image-upload");
    if (fileInput) fileInput.value = "";
  };

  const handleSubmit = async (e) => {
    e.preventDefault();

    if (!content.trim()) {
      toast.error("Please write something");
      return;
    }

    if (privacy === "private" && selectedViewerIds.length === 0) {
      toast.error("Pick at least one follower who can see this post");
      return;
    }

    setLoading(true);

    try {
      const formData = new FormData();
      formData.append("content", content);
      formData.append("privacy_level", privacy);
      if (privacy === "private") {
        selectedViewerIds.forEach((id) =>
          formData.append("allowed_user_ids", id),
        );
      }
      if (image) {
        formData.append("image", image);
      }

      const { ok, data } = await apiFetch("/post/create", {
        method: "POST",
        body: formData,
        // Don't set Content-Type header - let the browser set it with boundary
      });

      if (!ok) {
        throw new Error(data.error || data.message || "Failed to create post");
      }

      toast.success("Post created successfully!");
      setContent("");
      setPrivacy("public");
      setSelectedViewerIds([]);
      removeImage();

      if (onPostCreated) {
        onPostCreated(data.post || data);
      }
    } catch (error) {
      console.error("Error creating post:", error);
      toast.error(error.message || "Failed to create post");
    } finally {
      setLoading(false);
    }
  };

  return (
    <form
      onSubmit={handleSubmit}
      className='bg-white rounded-lg shadow p-4 mb-4'>
      <div className='flex items-start space-x-3'>
        <div className='w-10 h-10 rounded-full bg-blue-500 flex items-center justify-center text-white font-bold'>
          {user?.first_name?.[0] || "U"}
        </div>
        <div className='flex-1'>
          <textarea
            id='post-content'
            name='content'
            value={content}
            onChange={(e) => setContent(e.target.value)}
            placeholder="What's on your mind?"
            rows='3'
            className='w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 resize-none'
            disabled={loading}
          />

          {/* Image Preview */}
          {imagePreview && (
            <div className='relative mt-2'>
              <img
                src={imagePreview}
                alt='Preview'
                className='max-h-48 rounded-lg object-cover border'
              />
              <button
                type='button'
                onClick={removeImage}
                className='absolute top-2 right-2 bg-red-500 text-white rounded-full p-1 hover:bg-red-600 transition'>
                ×
              </button>
            </div>
          )}

          <div className='flex items-center justify-between mt-2'>
            <div className='flex items-center space-x-2'>
              {/* Image Upload Button */}
              <label className='cursor-pointer text-gray-500 hover:text-blue-500 transition'>
                <input
                  id='image-upload'
                  type='file'
                  accept='image/jpeg,image/png,image/gif'
                  onChange={handleImageChange}
                  className='hidden'
                  disabled={loading}
                />
                📷 <span className='text-sm'>Photo</span>
              </label>

              {/* Privacy Selector */}
              <select
                id='post-privacy'
                name='privacy'
                value={privacy}
                onChange={(e) => handlePrivacyChange(e.target.value)}
                className='px-3 py-1 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-sm'
                disabled={loading}>
                <option value='public'>🌍 Public</option>
                <option value='almost_private'>👥 Almost Private</option>
                <option value='private'>🔒 Private</option>
              </select>
            </div>

            <button
              type='submit'
              disabled={!content.trim() || loading}
              className='px-4 py-2 bg-blue-500 text-white rounded-lg hover:bg-blue-600 transition disabled:opacity-50 disabled:cursor-not-allowed'>
              {loading ? "Posting..." : "Post"}
            </button>
          </div>

          {/* Viewer picker - only for 'private' posts, restricted to the
              author's own followers (only they're eligible to be granted
              access to a private post). */}
          {privacy === "private" && (
            <div className='mt-2 p-3 border border-gray-200 rounded-lg bg-gray-50'>
              <p className='text-sm font-medium text-gray-700 mb-2'>
                Who can see this post?
              </p>
              {loadingFollowers ? (
                <p className='text-sm text-gray-400'>Loading followers...</p>
              ) : followers.length === 0 ? (
                <p className='text-sm text-gray-400'>
                  You don&apos;t have any followers yet, so nobody else can be
                  picked - only you will see this post.
                </p>
              ) : (
                <div className='space-y-1 max-h-40 overflow-y-auto'>
                  {followers.map(({ user: follower }) => (
                    <label
                      key={follower.id}
                      className='flex items-center space-x-2 text-sm cursor-pointer'>
                      <input
                        type='checkbox'
                        checked={selectedViewerIds.includes(follower.id)}
                        onChange={() => toggleViewer(follower.id)}
                        className='w-4 h-4 text-blue-600 border-gray-300 rounded focus:ring-blue-500'
                      />
                      <span>
                        {follower.first_name} {follower.last_name}
                        {follower.nickname && (
                          <span className='text-gray-400'>
                            {" "}
                            @{follower.nickname}
                          </span>
                        )}
                      </span>
                    </label>
                  ))}
                </div>
              )}
            </div>
          )}
        </div>
      </div>
    </form>
  );
}
