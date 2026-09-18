import {
  isValidEmail,
  isPasswordValid,
  getPasswordChecks,
  isValidNicknameFormat,
  getInitials,
  getAvatarSrc,
  debounce,
} from "./helpers";

describe("isValidEmail", () => {
  // Mirrors backend/pkg/handlers/auth_validation_test.go's TestIsValidEmail
  // cases where the two implementations actually agree - see the note
  // below on where they intentionally differ.
  test.each([
    ["a@b.com", true],
    ["first.last@sub.example.co", true],
    ["no-at-sign.com", false],
    ["trailing-at@", false],
    ["no-dot@domain", false],
    ["", false],
    ["a@b", false],
  ])("isValidEmail(%j) = %p", (email, want) => {
    expect(isValidEmail(email)).toBe(want);
  });

  // The frontend's regex additionally rejects whitespace and an empty
  // local part before "@" - stricter than the backend's isValidEmail,
  // which only checks for an "@" and a "." after it. Both reject a
  // genuinely malformed address, but a value could exist that one accepts
  // and the other doesn't (e.g. "@no-local-part.com" - accepted by the
  // backend's looser check, rejected here). Documented, not treated as a
  // bug: the frontend gate is advisory (fast feedback), the backend gate
  // is the one that's actually enforced.
  test("rejects a leading '@' with no local part (backend's looser check accepts this)", () => {
    expect(isValidEmail("@no-local-part.com")).toBe(false);
  });
});

describe("password validation", () => {
  // Mirrors backend/pkg/handlers/auth_validation_test.go's TestIsValidPassword.
  test.each([
    ["valid 12 chars all categories", "Sup3rSec9!zx", true],
    ["16 chars, over max", "Sup3rSecure9!zxy", false],
    ["exactly 15 chars (max)", "Sup3rSecure9!zx", true],
    ["11 chars, under min", "Sup3rSec9!z", false],
    ["missing uppercase", "sup3rsec9!zx", false],
    ["missing lowercase", "SUP3RSEC9!ZX", false],
    ["missing digit", "SuperSecure!zx", false],
    ["missing special char", "Sup3rSecure9zx", false],
    ["empty", "", false],
  ])("isPasswordValid(%s) = %p", (_name, password, want) => {
    expect(isPasswordValid(password)).toBe(want);
  });

  test("getPasswordChecks reports which specific requirement is missing", () => {
    const checks = getPasswordChecks("sup3rsec9!zx"); // valid except uppercase
    expect(checks.uppercase).toBe(false);
    expect(checks.lowercase).toBe(true);
    expect(checks.digit).toBe(true);
    expect(checks.special).toBe(true);
    expect(checks.length).toBe(true);
  });

  test("getPasswordChecks flags a disallowed character", () => {
    const checks = getPasswordChecks("Sup3rSec9!zé"); // é is outside every category
    expect(checks.validCharsOnly).toBe(false);
  });
});

describe("isValidNicknameFormat", () => {
  // Mirrors backend/pkg/handlers/auth_validation_test.go's TestIsValidNicknameFormat.
  test.each([
    ["abc", true],
    ["Abc123", true],
    ["fourteenChars1", true],
    ["fifteenChars123", false],
    ["ab", false],
    ["", false],
    ["has space", false],
    ["has-dash", false],
    ["has_underscore", false],
  ])("isValidNicknameFormat(%j) = %p", (nickname, want) => {
    expect(isValidNicknameFormat(nickname)).toBe(want);
  });
});

describe("getInitials", () => {
  test("both names present", () => {
    expect(getInitials("Jane", "Doe")).toBe("JD");
  });
  test("only first name", () => {
    expect(getInitials("Jane", "")).toBe("J");
  });
  test("neither name present", () => {
    expect(getInitials("", "")).toBe("?");
    expect(getInitials(undefined, undefined)).toBe("?");
  });
});

describe("getAvatarSrc", () => {
  test("returns null for no avatar", () => {
    expect(getAvatarSrc(null)).toBeNull();
    expect(getAvatarSrc(undefined)).toBeNull();
  });
  test("passes an absolute URL through unchanged", () => {
    const url = "https://api.dicebear.com/10.x/clay/svg?seed=Nova";
    expect(getAvatarSrc(url)).toBe(url);
  });
  test("prefixes a relative path with the backend origin", () => {
    expect(getAvatarSrc("/uploads/abc.jpg")).toBe(
      "http://localhost:8081/uploads/abc.jpg",
    );
  });
  test("accepts a custom backend origin", () => {
    expect(getAvatarSrc("/uploads/abc.jpg", "https://example.com")).toBe(
      "https://example.com/uploads/abc.jpg",
    );
  });
});

describe("debounce", () => {
  beforeEach(() => jest.useFakeTimers());
  afterEach(() => jest.useRealTimers());

  test("only invokes the function once after the delay, with the last call's args", () => {
    const fn = jest.fn();
    const debounced = debounce(fn, 300);

    debounced("first");
    debounced("second");
    debounced("third");

    expect(fn).not.toHaveBeenCalled();
    jest.advanceTimersByTime(300);

    expect(fn).toHaveBeenCalledTimes(1);
    expect(fn).toHaveBeenCalledWith("third");
  });
});
