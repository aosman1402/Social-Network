"use client";

import { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import toast from "react-hot-toast";

export default function GroupsPage() {
  const router = useRouter();
  const [groups, setGroups] = useState([]);
  const [loading, setLoading] = useState(true);
  const [showCreateForm, setShowCreateForm] = useState(false);
  const [newGroup, setNewGroup] = useState({ title: "", description: "" });
  const [creating, setCreating] = useState(false);

  useEffect(() => {
    fetchGroups();
  }, []);

  const fetchGroups = async () => {
    try {
      const authRes = await fetch("http://localhost:8081/api/check-auth", {
        credentials: "include",
      });
      if (!authRes.ok) {
        router.push("/login");
        return;
      }

      const response = await fetch("http://localhost:8081/api/groups", {
        credentials: "include",
      });
      if (response.ok) {
        const data = await response.json();
        setGroups(Array.isArray(data) ? data : []);
      } else {
        setGroups([]);
      }
    } catch (error) {
      console.error("Error fetching groups:", error);
      setGroups([]);
    } finally {
      setLoading(false);
    }
  };

  const handleCreateGroup = async (e) => {
    e.preventDefault();
    if (!newGroup.title.trim()) {
      toast.error("Group title is required");
      return;
    }

    setCreating(true);
    try {
      const response = await fetch("http://localhost:8081/api/group/create", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        credentials: "include",
        body: JSON.stringify({
          title: newGroup.title,
          description: newGroup.description || null,
        }),
      });

      const data = await response.json();

      if (!response.ok) {
        throw new Error(data.error || data.message || "Failed to create group");
      }

      toast.success("Group created successfully!");
      setNewGroup({ title: "", description: "" });
      setShowCreateForm(false);
      // Refresh groups list
      await fetchGroups();
      // Navigate to the new group
      if (data.group_id) {
        router.push(`/groups/${data.group_id}`);
      }
    } catch (error) {
      console.error("Error creating group:", error);
      toast.error(error.message || "Failed to create group");
    } finally {
      setCreating(false);
    }
  };

  const handleJoinGroup = async (groupId) => {
    try {
      const response = await fetch("http://localhost:8081/api/group/join", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        credentials: "include",
        body: JSON.stringify({ group_id: groupId }),
      });

      const data = await response.json();

      if (response.ok) {
        toast.success(data.message || "Request sent successfully");
        // Refresh groups
        await fetchGroups();
      } else {
        toast.error(data.error || "Failed to join group");
      }
    } catch (error) {
      console.error("Error joining group:", error);
      toast.error("Failed to join group");
    }
  };

  if (loading) {
    return (
      <div className='flex justify-center items-center h-64'>
        <div className='animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600'></div>
      </div>
    );
  }

  return (
    <div className='max-w-4xl mx-auto py-4 px-4'>
      <div className='flex items-center justify-between mb-4'>
        <h1 className='text-2xl font-bold'>Groups</h1>
        <button
          onClick={() => setShowCreateForm(!showCreateForm)}
          className='px-4 py-2 bg-blue-500 text-white rounded-lg hover:bg-blue-600 transition'>
          {showCreateForm ? "Cancel" : "+ Create Group"}
        </button>
      </div>

      {/* Create Group Form */}
      {showCreateForm && (
        <form
          onSubmit={handleCreateGroup}
          className='bg-white rounded-lg shadow p-4 mb-4'>
          <input
            type='text'
            placeholder='Group Title *'
            value={newGroup.title}
            onChange={(e) =>
              setNewGroup({ ...newGroup, title: e.target.value })
            }
            className='w-full px-3 py-2 border rounded-lg mb-2 focus:outline-none focus:ring-2 focus:ring-blue-500'
            required
          />
          <textarea
            placeholder='Description (optional)'
            value={newGroup.description}
            onChange={(e) =>
              setNewGroup({ ...newGroup, description: e.target.value })
            }
            rows='3'
            className='w-full px-3 py-2 border rounded-lg mb-2 focus:outline-none focus:ring-2 focus:ring-blue-500 resize-none'
          />
          <div className='flex justify-end'>
            <button
              type='submit'
              disabled={creating || !newGroup.title.trim()}
              className='px-4 py-2 bg-blue-500 text-white rounded-lg hover:bg-blue-600 transition disabled:opacity-50'>
              {creating ? "Creating..." : "Create Group"}
            </button>
          </div>
        </form>
      )}

      {/* Groups List */}
      <div className='grid grid-cols-1 md:grid-cols-2 gap-4'>
        {groups.length === 0 ? (
          <div className='col-span-2 bg-white rounded-lg shadow p-8 text-center'>
            <p className='text-gray-500'>No groups found</p>
            <p className='text-sm text-gray-400 mt-2'>
              Create your first group!
            </p>
          </div>
        ) : (
          groups.map((group) => (
            <div
              key={group.id}
              onClick={() => router.push(`/groups/${group.id}`)}
              className='bg-white rounded-lg shadow p-4 cursor-pointer hover:shadow-md transition'>
              <h3 className='font-semibold text-lg'>{group.title}</h3>
              <p className='text-gray-600 text-sm mt-1'>
                {group.description || "No description"}
              </p>
              <div className='flex items-center justify-between mt-2'>
                <span className='text-sm text-gray-500'>
                  👥 {group.member_count || 0} members
                </span>
                {!group.is_member &&
                  !group.has_pending_request &&
                  !group.is_invited && (
                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        handleJoinGroup(group.id);
                      }}
                      className='text-sm text-blue-500 hover:text-blue-700'>
                      Join
                    </button>
                  )}
                {group.has_pending_request && (
                  <span className='text-sm text-yellow-500'>
                    Request pending
                  </span>
                )}
                {group.is_invited && (
                  <span className='text-sm text-green-500'>Invited</span>
                )}
              </div>
            </div>
          ))
        )}
      </div>
    </div>
  );
}
