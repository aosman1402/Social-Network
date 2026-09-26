import { localInputToRFC3339, nowAsLocalInput, splitResponses } from "./events";

describe("localInputToRFC3339", () => {
  test("keeps the local wall-clock time and adds the local offset", () => {
    const out = localInputToRFC3339("2026-10-01T18:30");
    expect(out).toMatch(/^2026-10-01T18:30:00[+-]\d{2}:\d{2}$/);
  });

  test("refers to the same instant as the local input", () => {
    const out = localInputToRFC3339("2026-10-01T18:30");
    expect(new Date(out).getTime()).toBe(new Date("2026-10-01T18:30").getTime());
  });

  test("returns null for empty or invalid input", () => {
    expect(localInputToRFC3339("")).toBeNull();
    expect(localInputToRFC3339("not a date")).toBeNull();
  });
});

describe("nowAsLocalInput", () => {
  test("formats as a datetime-local value", () => {
    expect(nowAsLocalInput(new Date(2026, 0, 5, 9, 7))).toBe("2026-01-05T09:07");
  });
});

describe("splitResponses", () => {
  test("groups responses by answer", () => {
    const { going, notGoing } = splitResponses([
      { response: "going", user: { id: 1 } },
      { response: "not_going", user: { id: 2 } },
      { response: "going", user: { id: 3 } },
    ]);
    expect(going.map((r) => r.user.id)).toEqual([1, 3]);
    expect(notGoing.map((r) => r.user.id)).toEqual([2]);
  });

  test("handles a missing list", () => {
    expect(splitResponses(undefined)).toEqual({ going: [], notGoing: [] });
  });
});
