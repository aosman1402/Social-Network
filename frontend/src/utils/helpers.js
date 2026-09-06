/**
 * Format a date to a readable string
 * @param {string|Date} date - Date to format
 * @param {string} format - Format style: 'short', 'long', 'relative', 'time'
 * @returns {string} Formatted date string
 */
export const formatDate = (date, format = "relative") => {
  if (!date) return "N/A";

  const d = new Date(date);
  if (isNaN(d.getTime())) return "Invalid date";

  const now = new Date();
  const diff = now.getTime() - d.getTime();
  const seconds = Math.floor(diff / 1000);
  const minutes = Math.floor(seconds / 60);
  const hours = Math.floor(minutes / 60);
  const days = Math.floor(hours / 24);
  const months = Math.floor(days / 30);
  const years = Math.floor(days / 365);

  switch (format) {
    case "relative":
      if (seconds < 60) return "Just now";
      if (minutes < 60) return `${minutes}m ago`;
      if (hours < 24) return `${hours}h ago`;
      if (days < 7) return `${days}d ago`;
      if (days < 30) return `${Math.floor(days / 7)}w ago`;
      if (months < 12) return `${months}mo ago`;
      return `${years}y ago`;

    case "short":
      return d.toLocaleDateString("en-US", {
        month: "short",
        day: "numeric",
        year: "numeric",
      });

    case "long":
      return d.toLocaleDateString("en-US", {
        weekday: "long",
        month: "long",
        day: "numeric",
        year: "numeric",
        hour: "2-digit",
        minute: "2-digit",
      });

    case "time":
      return d.toLocaleTimeString("en-US", {
        hour: "2-digit",
        minute: "2-digit",
      });

    default:
      return d.toLocaleString();
  }
};

/**
 * Truncate text to a maximum length
 * @param {string} text - Text to truncate
 * @param {number} maxLength - Maximum length
 * @param {string} suffix - Suffix to add (default: '...')
 * @returns {string} Truncated text
 */
export const truncateText = (text, maxLength = 100, suffix = "...") => {
  if (!text) return "";
  if (text.length <= maxLength) return text;
  return text.substring(0, maxLength) + suffix;
};

/**
 * Capitalize the first letter of a string
 * @param {string} str - String to capitalize
 * @returns {string} Capitalized string
 */
export const capitalize = (str) => {
  if (!str) return "";
  return str.charAt(0).toUpperCase() + str.slice(1).toLowerCase();
};

/**
 * Generate a random string (for IDs, keys, etc.)
 * @param {number} length - Length of the string
 * @returns {string} Random string
 */
export const generateId = (length = 8) => {
  const chars =
    "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789";
  let result = "";
  for (let i = 0; i < length; i++) {
    result += chars.charAt(Math.floor(Math.random() * chars.length));
  }
  return result;
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
 * Throttle a function call
 * @param {Function} fn - Function to throttle
 * @param {number} limit - Limit in milliseconds
 * @returns {Function} Throttled function
 */
export const throttle = (fn, limit = 300) => {
  let inThrottle = false;
  return (...args) => {
    if (!inThrottle) {
      fn(...args);
      inThrottle = true;
      setTimeout(() => (inThrottle = false), limit);
    }
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

/**
 * Check if a string is a valid URL
 * @param {string} url - URL to validate
 * @returns {boolean} True if valid URL
 */
export const isValidUrl = (url) => {
  if (!url) return false;
  try {
    new URL(url);
    return true;
  } catch {
    return false;
  }
};

/**
 * Parse JSON safely
 * @param {string} json - JSON string to parse
 * @param {*} fallback - Fallback value if parsing fails
 * @returns {*} Parsed JSON or fallback
 */
export const safeJsonParse = (json, fallback = null) => {
  try {
    return JSON.parse(json);
  } catch {
    return fallback;
  }
};

/**
 * Get a nested object value safely
 * @param {Object} obj - Object to get value from
 * @param {string} path - Path to the value (dot notation)
 * @param {*} fallback - Fallback value if path doesn't exist
 * @returns {*} Value at path or fallback
 */
export const getNestedValue = (obj, path, fallback = null) => {
  if (!obj || !path) return fallback;
  const keys = path.split(".");
  let result = obj;
  for (const key of keys) {
    if (result === undefined || result === null) return fallback;
    result = result[key];
  }
  return result !== undefined ? result : fallback;
};

/**
 * Deep clone an object
 * @param {Object} obj - Object to clone
 * @returns {Object} Cloned object
 */
export const deepClone = (obj) => {
  if (obj === null || typeof obj !== "object") return obj;
  if (obj instanceof Date) return new Date(obj.getTime());
  if (obj instanceof Array) return obj.map((item) => deepClone(item));
  if (obj instanceof Object) {
    const cloned = {};
    for (const key in obj) {
      if (Object.prototype.hasOwnProperty.call(obj, key)) {
        cloned[key] = deepClone(obj[key]);
      }
    }
    return cloned;
  }
  return obj;
};

/**
 * Convert a file size to a readable string
 * @param {number} bytes - File size in bytes
 * @param {number} decimals - Number of decimal places
 * @returns {string} Readable file size
 */
export const formatFileSize = (bytes, decimals = 2) => {
  if (bytes === 0) return "0 Bytes";
  const k = 1024;
  const sizes = ["Bytes", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  const size = (bytes / Math.pow(k, i)).toFixed(decimals);
  return `${size} ${sizes[i]}`;
};

/**
 * Check if a value is empty (null, undefined, empty string, empty array, empty object)
 * @param {*} value - Value to check
 * @returns {boolean} True if empty
 */
export const isEmpty = (value) => {
  if (value === null || value === undefined) return true;
  if (typeof value === "string") return value.trim() === "";
  if (Array.isArray(value)) return value.length === 0;
  if (typeof value === "object") return Object.keys(value).length === 0;
  return false;
};

/**
 * Generate a slug from a string
 * @param {string} str - String to slugify
 * @returns {string} Slugified string
 */
export const slugify = (str) => {
  if (!str) return "";
  return str
    .toLowerCase()
    .replace(/[^a-z0-9\s-]/g, "")
    .replace(/\s+/g, "-")
    .replace(/-+/g, "-")
    .trim();
};

/**
 * Get a random color (for avatars, tags, etc.)
 * @returns {string} Hex color code
 */
export const getRandomColor = () => {
  const colors = [
    "#FF6B6B",
    "#4ECDC4",
    "#45B7D1",
    "#96CEB4",
    "#FFEAA7",
    "#DDA0DD",
    "#98D8C8",
    "#F7DC6F",
    "#BB8FCE",
    "#85C1E9",
    "#F8C471",
    "#82E0AA",
    "#F1948A",
    "#85929E",
    "#73C6B6",
  ];
  return colors[Math.floor(Math.random() * colors.length)];
};

/**
 * Get the difference between two dates in a readable format
 * @param {Date} date1 - First date
 * @param {Date} date2 - Second date
 * @returns {Object} Difference object with years, months, days, hours, minutes, seconds
 */
export const getDateDifference = (date1, date2) => {
  const d1 = new Date(date1);
  const d2 = new Date(date2);
  const diff = Math.abs(d2.getTime() - d1.getTime());

  const seconds = Math.floor(diff / 1000);
  const minutes = Math.floor(seconds / 60);
  const hours = Math.floor(minutes / 60);
  const days = Math.floor(hours / 24);
  const months = Math.floor(days / 30);
  const years = Math.floor(days / 365);

  return { years, months, days, hours, minutes, seconds };
};

/**
 * Format a phone number
 * @param {string} phone - Phone number to format
 * @returns {string} Formatted phone number
 */
export const formatPhone = (phone) => {
  if (!phone) return "";
  const cleaned = phone.replace(/\D/g, "");
  if (cleaned.length === 10) {
    return `(${cleaned.slice(0, 3)}) ${cleaned.slice(3, 6)}-${cleaned.slice(6)}`;
  }
  if (cleaned.length === 11) {
    return `+${cleaned.slice(0, 1)} (${cleaned.slice(1, 4)}) ${cleaned.slice(4, 7)}-${cleaned.slice(7)}`;
  }
  return phone;
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

/**
 * Get the first and last name from a full name
 * @param {string} fullName - Full name
 * @returns {Object} Object with firstName and lastName
 */
export const splitName = (fullName) => {
  if (!fullName) return { firstName: "", lastName: "" };
  const parts = fullName.trim().split(" ");
  if (parts.length === 1) {
    return { firstName: parts[0], lastName: "" };
  }
  return {
    firstName: parts[0],
    lastName: parts.slice(1).join(" "),
  };
};

/**
 * Check if running in browser environment
 * @returns {boolean} True if in browser
 */
export const isBrowser = () => {
  return typeof window !== "undefined";
};

/**
 * Check if running in development environment
 * @returns {boolean} True if in development
 */
export const isDevelopment = () => {
  return process.env.NODE_ENV === "development";
};

/**
 * Check if running in production environment
 * @returns {boolean} True if in production
 */
export const isProduction = () => {
  return process.env.NODE_ENV === "production";
};

// Export all functions as a single object for convenience
export default {
  formatDate,
  truncateText,
  capitalize,
  generateId,
  debounce,
  throttle,
  getInitials,
  getAvatarSrc,
  resizeImageFile,
  getPasswordChecks,
  isPasswordValid,
  PASSWORD_SPECIAL_CHARS,
  isValidNicknameFormat,
  isValidEmail,
  isValidUrl,
  safeJsonParse,
  getNestedValue,
  deepClone,
  formatFileSize,
  isEmpty,
  slugify,
  getRandomColor,
  getDateDifference,
  formatPhone,
  splitName,
  isBrowser,
  isDevelopment,
  isProduction,
};
