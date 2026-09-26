import {
  mergeMessages,
  insertAtCursor,
  charCount,
  confirmPendingMessage,
  dropPendingMessage,
} from "./chat";

const msg = (id, created_at, content = `m${id}`) => ({ id, created_at, content });

describe("mergeMessages", () => {
  test("drops a message that is already shown", () => {
    const shown = [msg(1, "2026-01-01T10:00:00Z")];
    const merged = mergeMessages(shown, [msg(1, "2026-01-01T10:00:00Z")]);
    expect(merged).toHaveLength(1);
  });

  test("drops duplicates within the incoming batch", () => {
    const m = msg(2, "2026-01-01T10:00:00Z");
    expect(mergeMessages([], [m, m])).toHaveLength(1);
  });

  test("sorts oldest to newest, e.g. a newest-first history page", () => {
    const history = [
      msg(3, "2026-01-01T10:02:00Z"),
      msg(2, "2026-01-01T10:01:00Z"),
      msg(1, "2026-01-01T10:00:00Z"),
    ];
    expect(mergeMessages([], history).map((m) => m.id)).toEqual([1, 2, 3]);
  });

  test("keeps a live message that arrived before the history loaded", () => {
    const live = [msg(4, "2026-01-01T10:03:00Z")];
    const history = [msg(4, "2026-01-01T10:03:00Z"), msg(1, "2026-01-01T10:00:00Z")];
    expect(mergeMessages(live, history).map((m) => m.id)).toEqual([1, 4]);
  });

  test("breaks created_at ties by id", () => {
    const t = "2026-01-01T10:00:00Z";
    expect(mergeMessages([msg(6, t)], [msg(5, t)]).map((m) => m.id)).toEqual([5, 6]);
  });

  test("does not mutate its inputs", () => {
    const shown = [msg(2, "2026-01-01T10:01:00Z")];
    mergeMessages(shown, [msg(1, "2026-01-01T10:00:00Z")]);
    expect(shown.map((m) => m.id)).toEqual([2]);
  });
});

describe("insertAtCursor", () => {
  test("inserts at the cursor", () => {
    expect(insertAtCursor("helo", "l", 3)).toEqual({ value: "hello", cursor: 4 });
  });

  test("replaces a selection", () => {
    expect(insertAtCursor("hi there", "😀", 3, 8)).toEqual({ value: "hi 😀", cursor: 5 });
  });

  test("appends when there is no cursor position", () => {
    expect(insertAtCursor("hi", "!", undefined)).toEqual({ value: "hi!", cursor: 3 });
  });
});

describe("charCount", () => {
  test("counts an emoji as one character", () => {
    expect(charCount("hi 😀")).toBe(4);
  });
});

describe("confirmPendingMessage", () => {
  const pending = (clientId, content) => ({
    id: clientId,
    client_id: clientId,
    content,
    created_at: "2026-01-01T10:00:00Z",
    pending: true,
  });

  test("swaps the temporary id for the server's and clears pending", () => {
    const out = confirmPendingMessage([pending("tmp-1", "hi")], "tmp-1", {
      message_id: 99,
      created_at: "2026-01-01T10:00:01Z",
    });
    expect(out).toEqual([
      expect.objectContaining({ id: 99, content: "hi", pending: false, created_at: "2026-01-01T10:00:01Z" }),
    ]);
  });

  test("matches by client id even when acks arrive out of order", () => {
    const list = [pending("tmp-1", "first"), pending("tmp-2", "second")];
    const out = confirmPendingMessage(list, "tmp-2", { message_id: 11 });
    expect(out.map((m) => [m.content, m.id, m.pending])).toEqual([
      ["first", "tmp-1", true],
      ["second", 11, false],
    ]);
  });

  test("drops the optimistic copy if the real message is already shown", () => {
    const list = [{ id: 5, content: "hi" }, pending("tmp-1", "hi")];
    expect(confirmPendingMessage(list, "tmp-1", { message_id: 5 })).toEqual([{ id: 5, content: "hi" }]);
  });

  test("a second ack for the same message changes nothing", () => {
    const once = confirmPendingMessage([pending("tmp-1", "hi")], "tmp-1", { message_id: 3 });
    expect(confirmPendingMessage(once, "tmp-1", { message_id: 3 })).toEqual(once);
  });
});

describe("dropPendingMessage", () => {
  test("removes only the failed message", () => {
    const list = [
      { id: 1, content: "sent" },
      { id: "tmp-1", client_id: "tmp-1", pending: true },
      { id: "tmp-2", client_id: "tmp-2", pending: true },
    ];
    expect(dropPendingMessage(list, "tmp-1").map((m) => m.id)).toEqual([1, "tmp-2"]);
  });
});
