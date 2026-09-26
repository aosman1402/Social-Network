"use client";

import { useState, useEffect, useCallback } from "react";
import { apiFetch } from "@/lib/api";
import { toast } from "@/context/ToastContext";
import { useAuth } from "@/context/AuthContext";
import { useWebSocket } from "@/context/WebSocketContext";
import { localInputToRFC3339, nowAsLocalInput, splitResponses } from "@/utils/events";
import DeleteButton from "@/components/DeleteButton";

const RESPONSE_OPTIONS = [
  { value: "going", label: "Going" },
  { value: "not_going", label: "Not going" },
];

const EMPTY_FORM = { title: "", description: "", eventTime: "" };

const formatEventTime = (iso) =>
  new Date(iso).toLocaleString(undefined, {
    weekday: "short",
    day: "numeric",
    month: "short",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });

const fullName = (u) => `${u?.first_name || ""} ${u?.last_name || ""}`.trim();

// Events section of a group page, members only: a create form plus the
// group's events with Going / Not going voting. The list refreshes live
// when another member creates an event (the "event_created" notification
// for this group) or answers one ("event_response").
export default function GroupEvents({ groupId }) {
  const gid = parseInt(groupId);
  const { user } = useAuth();
  const { subscribe } = useWebSocket();

  const [events, setEvents] = useState([]);
  const [loading, setLoading] = useState(true);
  const [formOpen, setFormOpen] = useState(false);
  const [form, setForm] = useState(EMPTY_FORM);
  const [creating, setCreating] = useState(false);
  const [respondingId, setRespondingId] = useState(null);

  const fetchEvents = useCallback(async () => {
    try {
      const { ok, data } = await apiFetch(`/group/events?group_id=${gid}`);
      if (ok && Array.isArray(data)) setEvents(data);
    } catch (err) {
      console.error("Error fetching events:", err);
    } finally {
      setLoading(false);
    }
  }, [gid]);

  useEffect(() => {
    fetchEvents();
  }, [fetchEvents]);

  useEffect(() => {
    return subscribe("notification", (n) => {
      if (n.type === "event_created" && n.related_id === gid) fetchEvents();
    });
  }, [subscribe, gid, fetchEvents]);

  useEffect(() => {
    return subscribe("event_response", (evt) => {
      if (evt.group_id === gid) fetchEvents();
    });
  }, [subscribe, gid, fetchEvents]);

  // Another member's view: the creator cancelled an event.
  useEffect(() => {
    return subscribe("event_deleted", (evt) => {
      if (evt.group_id !== gid) return;
      setEvents((prev) => prev.filter((e) => e.id !== evt.event_id));
    });
  }, [subscribe, gid]);

  // The event's creator cancels it - same confirm-then-delete as a post.
  const handleDelete = async (eventId) => {
    if (!confirm("Are you sure you want to delete this event?")) return;
    try {
      const { ok, data } = await apiFetch(`/group/event/delete?id=${eventId}`, {
        method: "DELETE",
      });
      if (!ok) {
        toast.error(data?.error || "Failed to delete event");
        return;
      }
      setEvents((prev) => prev.filter((e) => e.id !== eventId));
      toast.success("Event deleted");
    } catch (err) {
      console.error("Error deleting event:", err);
      toast.error("Failed to delete event");
    }
  };

  const handleCreate = async (e) => {
    e.preventDefault();
    const title = form.title.trim();
    if (!title) {
      toast.error("Please give the event a title");
      return;
    }
    const eventTime = localInputToRFC3339(form.eventTime);
    if (!eventTime) {
      toast.error("Please pick a day and time");
      return;
    }
    if (new Date(eventTime) < new Date()) {
      toast.error("The event can't be in the past");
      return;
    }

    setCreating(true);
    try {
      const { ok, data } = await apiFetch("/group/event/create", {
        method: "POST",
        json: {
          group_id: gid,
          title,
          description: form.description.trim(),
          event_time: eventTime,
        },
      });
      if (!ok) {
        toast.error(data.error || "Failed to create event");
        return;
      }
      toast.success("Event created - members have been notified");
      setForm(EMPTY_FORM);
      setFormOpen(false);
      fetchEvents();
    } catch (err) {
      console.error("Error creating event:", err);
      toast.error("Failed to create event");
    } finally {
      setCreating(false);
    }
  };

  const handleRespond = async (eventId, response) => {
    setRespondingId(eventId);
    try {
      const { ok, data } = await apiFetch("/group/event/respond", {
        method: "POST",
        json: { event_id: eventId, response },
      });
      if (!ok) {
        toast.error(data.error || "Failed to save your answer");
        return;
      }
      fetchEvents();
    } catch (err) {
      console.error("Error responding to event:", err);
      toast.error("Failed to save your answer");
    } finally {
      setRespondingId(null);
    }
  };

  return (
    <div className='group-section' id='events'>
      <div className='group-events-header'>
        <h3 className='group-section-title group-events-title'>
          Events ({events.length})
        </h3>
        <button
          type='button'
          className='group-events-new-btn'
          onClick={() => setFormOpen((v) => !v)}
          aria-expanded={formOpen}>
          {formOpen ? "Cancel" : "+ New event"}
        </button>
      </div>

      {formOpen && (
        <form className='group-event-form' onSubmit={handleCreate}>
          <label className='group-event-label'>
            Title
            <input
              type='text'
              className='group-event-input'
              value={form.title}
              onChange={(e) => setForm({ ...form, title: e.target.value })}
              maxLength={100}
              required
              placeholder='e.g. Study session'
            />
          </label>
          <label className='group-event-label'>
            Description
            <textarea
              className='group-event-input group-event-textarea'
              value={form.description}
              onChange={(e) => setForm({ ...form, description: e.target.value })}
              maxLength={1000}
              rows={3}
              placeholder='What, where, what to bring...'
            />
          </label>
          <label className='group-event-label'>
            Day and time
            <input
              type='datetime-local'
              className='group-event-input'
              value={form.eventTime}
              min={nowAsLocalInput()}
              onChange={(e) => setForm({ ...form, eventTime: e.target.value })}
              required
            />
          </label>
          <div className='group-event-label'>
            Options members can choose
            <div className='group-event-options'>
              {RESPONSE_OPTIONS.map((opt) => (
                <span key={opt.value} className='group-event-option-chip'>
                  {opt.label}
                </span>
              ))}
            </div>
          </div>
          <button type='submit' className='group-event-submit' disabled={creating}>
            {creating ? "Creating..." : "Create event"}
          </button>
        </form>
      )}

      <div className='group-section-body'>
        {loading ? (
          <p className='group-empty'>Loading events...</p>
        ) : events.length === 0 ? (
          <p className='group-empty'>No events yet.</p>
        ) : (
          events.map((event) => {
            const { going, notGoing } = splitResponses(event.responses);
            const isPast = new Date(event.event_time) < new Date();
            return (
              <article
                key={event.id}
                className={`group-event-card ${isPast ? "group-event-card--past" : ""}`}>
                <div className='group-event-top'>
                  <h4 className='group-event-name'>{event.title}</h4>
                  <div className='group-event-top-actions'>
                    {isPast && <span className='group-badge'>Past</span>}
                    {event.creator?.id === user?.id && (
                      <DeleteButton
                        onClick={() => handleDelete(event.id)}
                        label={`Delete event ${event.title}`}
                      />
                    )}
                  </div>
                </div>
                <p className='group-event-when'>📅 {formatEventTime(event.event_time)}</p>
                {event.description && (
                  <p className='group-event-desc'>{event.description}</p>
                )}
                <p className='group-event-creator'>
                  Created by{" "}
                  {event.creator?.id === user?.id ? "you" : fullName(event.creator)}
                </p>

                <div className='group-event-actions'>
                  {RESPONSE_OPTIONS.map((opt) => {
                    const selected = event.user_response === opt.value;
                    return (
                      <button
                        key={opt.value}
                        type='button'
                        className={`group-event-vote ${
                          selected ? `group-event-vote--${opt.value}` : ""
                        }`}
                        aria-pressed={selected}
                        disabled={isPast || respondingId === event.id}
                        onClick={() => handleRespond(event.id, opt.value)}>
                        {selected ? "✓ " : ""}
                        {opt.label} ({opt.value === "going" ? going.length : notGoing.length})
                      </button>
                    );
                  })}
                </div>

                {(going.length > 0 || notGoing.length > 0) && (
                  <div className='group-event-who'>
                    {going.length > 0 && (
                      <p>
                        <strong>Going:</strong>{" "}
                        {going.map((r) => fullName(r.user)).join(", ")}
                      </p>
                    )}
                    {notGoing.length > 0 && (
                      <p>
                        <strong>Not going:</strong>{" "}
                        {notGoing.map((r) => fullName(r.user)).join(", ")}
                      </p>
                    )}
                  </div>
                )}
              </article>
            );
          })
        )}
      </div>
    </div>
  );
}
