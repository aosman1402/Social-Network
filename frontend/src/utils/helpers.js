/**
 * Map a notification's type to the icon (a components/Icon name) shown next to it. Shared between
 * the notifications page and the notification bell dropdown - previously
 * duplicated verbatim in both.
 * @param {string} type - Notification type (e.g. "follow_accepted")
 * @returns {string} An Icon name
 */
export const getNotificationIcon = (type) => {
  switch (type) {
    case "follow_request":
    case "follow_accepted":
    case "new_follower":
      return "user";
    case "follow_declined":
      return "user-x";
    case "unfollow":
      return "user-minus";
    case "group_invitation":
      return "users";
    case "group_request":
      return "mail";
    case "group_request_accepted":
    case "group_invitation_accepted":
      return "check-circle";
    case "group_invitation_declined":
    case "group_request_declined":
      return "user-x";
    case "event_created":
      return "calendar";
    case "comment":
      return "message";
    case "profile_public":
      return "globe";
    case "profile_private":
      return "lock";
    default:
      return "bell";
  }
};

/**
 * Map a notification to the URL clicking it should navigate to. Shared
 * between the notifications page and the notification bell dropdown -
 * previously each had its own, independently-drifted switch statement.
 * @param {object} notification - Notification object (type, related_id, post_id)
 * @param {number|string} currentUserId - The signed-in user's id
 * @returns {string} A path to navigate to
 */
export const getNotificationTarget = (notification, currentUserId) => {
  const { type, related_id: relatedId, post_id: postId } = notification;
  switch (type) {
    case "follow_request":
      // related_id here is the sender (the person who wants to follow
      // you) - the recipient needs to land on their OWN profile's
      // Requests tab to accept/decline, not on the sender's profile.
      return `/profile/${currentUserId}?tab=requests`;
    case "follow_accepted":
    case "follow_declined":
    case "new_follower":
    case "unfollow":
      return `/profile/${relatedId}`;
    case "group_invitation":
    case "group_request":
    case "group_request_accepted":
    case "group_invitation_accepted":
    case "group_invitation_declined":
    case "group_request_declined":
      return `/groups/${relatedId}`;
    case "event_created":
      return `/groups/${relatedId}#events`;
    case "like":
      return `/?post=${postId}`;
    case "comment":
      return `/?post=${postId}&comment=${relatedId}`;
    case "profile_public":
    case "profile_private":
      return `/profile/${currentUserId}`;
    default:
      return "/notifications";
  }
};

/**
 * Debounce a function call
 * @param {Function} fn - Function to debounce
 * @param {number} delay - Delay in milliseconds
 * @returns {Function} Debounced function
 */
export const debounce = (fn, delay = 300) => {
  let timeoutId;
  return (...args) => {
    clearTimeout(timeoutId);
    timeoutId = setTimeout(() => fn(...args), delay);
  };
};

/**
 * Get initials from a name
 * @param {string} firstName - First name
 * @param {string} lastName - Last name
 * @returns {string} Initials (max 2 characters)
 */
export const getInitials = (firstName, lastName) => {
  if (!firstName && !lastName) return "?";
  const first = firstName?.charAt(0) || "";
  const last = lastName?.charAt(0) || "";
  return (first + last).toUpperCase();
};

/**
 * Special/symbol characters the password policy accepts, exactly as listed:
 * the space character plus every standard printable-ASCII punctuation mark.
 */
export const PASSWORD_SPECIAL_CHARS = " !\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~";

/**
 * Check a password against the site's policy: 12-15 characters, made up
 * entirely of lowercase letters, uppercase letters, digits, and/or the
 * special characters above, with at least one of each of those four
 * categories present. Used to both drive the live requirements checklist
 * and to gate submission - the backend enforces the same rule independently.
 * @param {string} password - Password to check
 * @returns {{length: boolean, lowercase: boolean, uppercase: boolean, digit: boolean, special: boolean, validCharsOnly: boolean}}
 */
export const getPasswordChecks = (password = "") => {
  let validCharsOnly = true;
  let lowercase = false;
  let uppercase = false;
  let digit = false;
  let special = false;

  for (const ch of password) {
    if (ch >= "a" && ch <= "z") lowercase = true;
    else if (ch >= "A" && ch <= "Z") uppercase = true;
    else if (ch >= "0" && ch <= "9") digit = true;
    else if (PASSWORD_SPECIAL_CHARS.includes(ch)) special = true;
    else validCharsOnly = false;
  }

  return {
    length: password.length >= 12 && password.length <= 15,
    lowercase,
    uppercase,
    digit,
    special,
    validCharsOnly,
  };
};

/**
 * Whether a password satisfies every requirement in getPasswordChecks.
 * @param {string} password - Password to check
 * @returns {boolean} True if the password meets the full policy
 */
export const isPasswordValid = (password = "") =>
  Object.values(getPasswordChecks(password)).every(Boolean);

/**
 * Nickname format policy: 3-14 characters, using only letters (a-z, A-Z)
 * and digits (0-9). Doesn't check uniqueness - that requires asking the
 * server (see the register page's availability check).
 * @param {string} nickname - Nickname to check
 * @returns {boolean} True if the format is valid
 */
export const isValidNicknameFormat = (nickname = "") =>
  /^[A-Za-z0-9]{3,14}$/.test(nickname);

/**
 * Check if a string is a valid email
 * @param {string} email - Email to validate
 * @returns {boolean} True if valid email
 */
export const isValidEmail = (email) => {
  if (!email) return false;
  const regex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
  return regex.test(email);
};

// Gradients for avatars without a picture - from the site palette, all
// dark enough for white initials.
const AVATAR_GRADIENTS = [
  "linear-gradient(135deg, #6366f1, #8b5cf6)", // indigo -> violet
  "linear-gradient(135deg, #ec4899, #f43f5e)", // pink -> rose
  "linear-gradient(135deg, #0ea5e9, #6366f1)", // sky -> indigo
  "linear-gradient(135deg, #10b981, #0d9488)", // emerald -> teal
  "linear-gradient(135deg, #f59e0b, #ef4444)", // amber -> red
  "linear-gradient(135deg, #8b5cf6, #d946ef)", // violet -> fuchsia
  "linear-gradient(135deg, #14b8a6, #0ea5e9)", // teal -> sky
  "linear-gradient(135deg, #f97316, #db2777)", // orange -> pink
];

/**
 * Pick an avatar background for someone without a picture. The same seed
 * (e.g. their id or name) always gives the same colour, so a person's
 * initials don't change colour every time the page re-renders.
 * @param {string|number} seed
 * @returns {string} A CSS background value
 */
export const getAvatarColor = (seed) => {
  const str = String(seed ?? "");
  let hash = 0;
  for (let i = 0; i < str.length; i++) {
    hash = (hash * 31 + str.charCodeAt(i)) | 0;
  }
  return AVATAR_GRADIENTS[Math.abs(hash) % AVATAR_GRADIENTS.length];
};

/**
 * Resolve a stored avatar_path into something an <img> can load directly.
 * An absolute URL (e.g. a DiceBear avatar picked at registration) is
 * returned as-is; a relative path (e.g. "/uploads/xxx.jpg", from an
 * uploaded file) is prefixed with the backend origin.
 * @param {string|null|undefined} avatarPath - Stored avatar_path value
 * @param {string} backendOrigin - Backend origin to prefix relative paths with
 * @returns {string|null} Usable image src, or null if there's no avatar
 */
export const getAvatarSrc = (avatarPath, backendOrigin = "http://localhost:8081") => {
  if (!avatarPath) return null;
  if (/^https?:\/\//i.test(avatarPath)) return avatarPath;
  return `${backendOrigin}${avatarPath}`;
};

/**
 * Downscale and re-encode an image file in the browser before upload, so it
 * doesn't take much space once stored. GIFs are returned unchanged - running
 * one through <canvas> would flatten its animation to a single frame - and
 * rely on the backend's own size cap instead.
 * @param {File} file - Original image file (already validated as JPEG/PNG/GIF)
 * @param {number} maxDimension - Max width/height in pixels
 * @param {number} quality - JPEG re-encode quality (0-1); ignored for PNG/GIF
 * @returns {Promise<Blob>} The resized (or, for GIF, unchanged) image blob
 */
export const resizeImageFile = (file, maxDimension = 320, quality = 0.8) => {
  return new Promise((resolve, reject) => {
    if (file.type === "image/gif") {
      resolve(file);
      return;
    }

    const img = new Image();
    const objectUrl = URL.createObjectURL(file);

    img.onload = () => {
      URL.revokeObjectURL(objectUrl);

      const scale = Math.min(1, maxDimension / Math.max(img.width, img.height));
      const width = Math.round(img.width * scale);
      const height = Math.round(img.height * scale);

      const canvas = document.createElement("canvas");
      canvas.width = width;
      canvas.height = height;
      canvas.getContext("2d").drawImage(img, 0, 0, width, height);

      const outputType = file.type === "image/png" ? "image/png" : "image/jpeg";
      canvas.toBlob(
        (blob) =>
          blob ? resolve(blob) : reject(new Error("Failed to process image")),
        outputType,
        outputType === "image/jpeg" ? quality : undefined
      );
    };
    img.onerror = () => {
      URL.revokeObjectURL(objectUrl);
      reject(new Error("Failed to load image"));
    };
    img.src = objectUrl;
  });
};
