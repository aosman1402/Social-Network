"use client";

import { useState, useRef } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { useAuth } from "@/context/AuthContext";
import { toast } from "@/context/ToastContext";
import {
  resizeImageFile,
  isValidEmail,
  isPasswordValid,
  isValidNicknameFormat,
  debounce,
} from "@/utils/helpers";
import PasswordRequirements from "@/components/PasswordRequirements";

// Preset avatar options, generated from DiceBear's "clay" style
// (https://www.dicebear.com/styles/clay/). Each preset pairs a fixed seed
// with one of the approved background colors so the six options are stable
// across renders instead of changing on every reload.
const AVATAR_BACKGROUND_COLORS = [
  "ff2e63",
  "00c2a8",
  "ffb300",
  "3d5afe",
  "8e24aa",
  "00e676",
];
const AVATAR_SEEDS = ["Nova", "Comet", "Lumen", "Aster", "Orbit", "Quartz"];
const DICEBEAR_AVATARS = AVATAR_SEEDS.map((seed, i) => ({
  seed,
  url: `https://api.dicebear.com/10.x/clay/svg?seed=${seed}&backgroundColor=${AVATAR_BACKGROUND_COLORS[i]}`,
}));

// Same allowlist the backend enforces (by sniffing the real file bytes) -
// checked here first so the user gets immediate feedback instead of
// waiting on a round trip for an upload that was always going to be rejected.
const ALLOWED_AVATAR_TYPES = ["image/jpeg", "image/png", "image/gif"];
const ALLOWED_AVATAR_EXTENSIONS = [".jpg", ".jpeg", ".png", ".gif"];

// Caps the date picker so a birth date can't be set in the future.
const todayIsoDate = new Date().toISOString().split("T")[0];

export default function RegisterPage() {
  const router = useRouter();
  const { login } = useAuth();
  const [formData, setFormData] = useState({
    email: "",
    password: "",
    confirmPassword: "",
    first_name: "",
    last_name: "",
    date_of_birth: "",
    nickname: "",
    about_me: "",
  });
  const [errors, setErrors] = useState({});
  const [isLoading, setIsLoading] = useState(false);
  const [serverError, setServerError] = useState("");
  const [debugInfo, setDebugInfo] = useState("");

  // Avatar/Image (Optional): "none" | "dicebear" | "upload"
  const [avatarChoice, setAvatarChoice] = useState("none");
  const [dicebearUrl, setDicebearUrl] = useState("");
  const [avatarFile, setAvatarFile] = useState(null); // resized Blob, ready to upload
  const [avatarPreviewUrl, setAvatarPreviewUrl] = useState("");
  const [avatarError, setAvatarError] = useState("");

  // Nickname (Optional): "idle" | "invalid" | "checking" | "available" | "taken" | "error"
  const [nicknameStatus, setNicknameStatus] = useState("idle");
  // Tracks the most recently typed value so a slow/late availability
  // response can't overwrite the status for a value the user has since
  // changed (or cleared) - checked before applying the response below.
  const latestNicknameRef = useRef("");

  const checkNicknameAvailability = useRef(
    debounce(async (nickname) => {
      try {
        const response = await fetch(
          `http://localhost:8081/api/check-nickname?nickname=${encodeURIComponent(nickname)}`,
          { credentials: "include" }
        );
        const data = await response.json();
        if (latestNicknameRef.current !== nickname) return; // superseded
        if (!data.valid) setNicknameStatus("invalid");
        else setNicknameStatus(data.available ? "available" : "taken");
      } catch (err) {
        if (latestNicknameRef.current !== nickname) return;
        setNicknameStatus("error");
      }
    }, 400)
  ).current;

  const handleNicknameChange = (e) => {
    const value = e.target.value;
    latestNicknameRef.current = value;
    setFormData((prev) => ({ ...prev, nickname: value }));
    if (errors.nickname) setErrors((prev) => ({ ...prev, nickname: "" }));
    setServerError("");
    setDebugInfo("");

    if (!value) {
      setNicknameStatus("idle");
      return;
    }
    if (!isValidNicknameFormat(value)) {
      setNicknameStatus("invalid");
      return;
    }
    setNicknameStatus("checking");
    checkNicknameAvailability(value);
  };

  const handleAvatarFileChange = async (e) => {
    const file = e.target.files[0];
    if (!file) return;

    const extension = file.name.slice(file.name.lastIndexOf(".")).toLowerCase();
    const isAllowedType =
      ALLOWED_AVATAR_TYPES.includes(file.type) ||
      ALLOWED_AVATAR_EXTENSIONS.includes(extension);

    if (!isAllowedType) {
      setAvatarError("Please choose a JPG, PNG, or GIF image.");
      setAvatarFile(null);
      setAvatarPreviewUrl("");
      e.target.value = "";
      return;
    }
    if (file.size > 5 * 1024 * 1024) {
      setAvatarError("Image must be less than 5MB.");
      e.target.value = "";
      return;
    }

    setAvatarError("");
    try {
      // Shrink dimensions/quality client-side before it ever reaches the
      // server, so a full-resolution photo doesn't end up sitting in storage.
      const resized = await resizeImageFile(file);
      if (avatarPreviewUrl) URL.revokeObjectURL(avatarPreviewUrl);
      setAvatarFile(resized);
      setAvatarPreviewUrl(URL.createObjectURL(resized));
    } catch (err) {
      setAvatarError("Failed to process that image. Please try a different file.");
    }
  };

  const validateForm = () => {
    const newErrors = {};
    if (!formData.email) newErrors.email = "Email is required";
    else if (!isValidEmail(formData.email))
      newErrors.email = "Please enter a valid email";
    if (!formData.password) newErrors.password = "Password is required";
    else if (!isPasswordValid(formData.password))
      newErrors.password = "Password does not meet all requirements below";
    if (formData.password !== formData.confirmPassword)
      newErrors.confirmPassword = "Passwords do not match";
    if (!formData.first_name) newErrors.first_name = "First name is required";
    if (!formData.last_name) newErrors.last_name = "Last name is required";
    if (!formData.date_of_birth)
      newErrors.date_of_birth = "Date of birth is required";
    // Nickname is optional - only enforced once the user has typed one.
    if (formData.nickname) {
      if (nicknameStatus === "invalid")
        newErrors.nickname =
          "Nickname must be 3-14 characters, letters and numbers only";
      else if (nicknameStatus === "taken")
        newErrors.nickname = "Nickname is already taken";
      else if (nicknameStatus === "checking")
        newErrors.nickname = "Please wait for the nickname check to finish";
      else if (nicknameStatus === "error")
        newErrors.nickname = "Could not verify nickname - please try again";
    }
    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  const handleChange = (e) => {
    const { name, value } = e.target;
    setFormData({ ...formData, [name]: value });
    if (errors[name]) setErrors({ ...errors, [name]: "" });
    setServerError("");
    setDebugInfo("");
  };

  const handleSubmit = async (e) => {
    e.preventDefault();
    if (!validateForm()) return;

    setIsLoading(true);
    setServerError("");
    setDebugInfo("");

    try {
      // The date input's value is already "YYYY-MM-DD" - no parsing needed.
      const payload = {
        email: formData.email,
        password: formData.password,
        first_name: formData.first_name,
        last_name: formData.last_name,
        date_of_birth: formData.date_of_birth,
        nickname: formData.nickname || null,
        about_me: formData.about_me || null,
        // A chosen DiceBear avatar is just a URL, so it can be saved directly
        // as avatar_path here. An uploaded file can't - the account doesn't
        // exist yet to attach it to - so that's sent separately, below, once
        // login succeeds.
        avatar_path: avatarChoice === "dicebear" ? dicebearUrl : null,
      };

      setDebugInfo("1. Sending registration request...");
      console.log("Sending registration:", payload);

      const response = await fetch("http://localhost:8081/api/register", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        credentials: "include",
        body: JSON.stringify(payload),
      });

      setDebugInfo(`2. Registration response status: ${response.status}`);
      console.log("Registration response status:", response.status);

      const data = await response.json();
      setDebugInfo(`3. Registration response: ${JSON.stringify(data)}`);
      console.log("Registration response:", data);

      if (!response.ok) {
        throw new Error(data.message || data.error || "Registration failed");
      }

      setDebugInfo("4. Registration successful! Auto-logging in...");
      console.log("Registration successful! Auto-logging in...");

      // ✅ Try to auto-login
      const loginResult = await login(formData.email, formData.password);
      console.log("Auto-login result:", loginResult);

      if (loginResult.success) {
        // The account (and its session) now exist, so an uploaded avatar can
        // finally be attached - reusing the same endpoint Settings uses.
        if (avatarChoice === "upload" && avatarFile) {
          try {
            const extensionByType = {
              "image/png": "png",
              "image/gif": "gif",
              "image/jpeg": "jpg",
            };
            const avatarForm = new FormData();
            avatarForm.append(
              "avatar",
              avatarFile,
              `avatar.${extensionByType[avatarFile.type] || "jpg"}`
            );
            const avatarResponse = await fetch(
              "http://localhost:8081/api/user/avatar",
              {
                method: "POST",
                credentials: "include",
                body: avatarForm,
              }
            );
            if (!avatarResponse.ok) {
              const avatarData = await avatarResponse.json().catch(() => ({}));
              toast.error(
                avatarData.error ||
                  "Account created, but the avatar upload failed. You can add one later in Settings."
              );
            }
          } catch (avatarErr) {
            console.error("Avatar upload error:", avatarErr);
            toast.error(
              "Account created, but the avatar upload failed. You can add one later in Settings."
            );
          }
        }

        setDebugInfo("5. Auto-login successful! Redirecting to home...");
        console.log("Auto-login successful! Redirecting to home...");
        // Force redirect with window.location
        window.location.href = "/";
      } else {
        setDebugInfo(`5. Auto-login failed: ${loginResult.error}`);
        console.error("Auto-login failed:", loginResult.error);
        // Fallback: redirect to login page
        router.push("/login?registered=true");
      }
    } catch (error) {
      console.error("Registration error:", error);
      setServerError(error.message || "Registration failed. Please try again.");
      setDebugInfo(`❌ Error: ${error.message}`);
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className='min-h-screen flex items-center justify-center bg-gray-50 py-12 px-4 sm:px-6 lg:px-8'>
      <div className='max-w-md w-full bg-white rounded-lg shadow-lg p-8'>
        <div className='text-center mb-8'>
          <h2 className='text-3xl font-bold text-gray-900'>Create Account</h2>
          <p className='text-gray-600 mt-2'>Join the social network</p>
        </div>

        {/* noValidate: the browser's own type="email" check would otherwise
            intercept submission before validateForm() runs, so its (less
            informative, inconsistent-across-browsers) tooltip would show
            instead of our own message. */}
        <form onSubmit={handleSubmit} noValidate className='space-y-4'>
          <div>
            <label
              htmlFor='register-email'
              className='block text-sm font-medium text-gray-700 mb-1'>
              Email *
            </label>
            <input
              id='register-email'
              type='email'
              name='email'
              value={formData.email}
              onChange={handleChange}
              className={`w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 ${
                errors.email ? "border-red-500" : "border-gray-300"
              }`}
              placeholder='Enter your email'
            />
            {errors.email && (
              <p className='text-red-500 text-sm mt-1'>{errors.email}</p>
            )}
          </div>

          <div className='grid grid-cols-2 gap-4'>
            <div>
              <label
                htmlFor='register-first-name'
                className='block text-sm font-medium text-gray-700 mb-1'>
                First Name *
              </label>
              <input
                id='register-first-name'
                type='text'
                name='first_name'
                value={formData.first_name}
                onChange={handleChange}
                className={`w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 ${
                  errors.first_name ? "border-red-500" : "border-gray-300"
                }`}
                placeholder='First name'
              />
              {errors.first_name && (
                <p className='text-red-500 text-sm mt-1'>{errors.first_name}</p>
              )}
            </div>
            <div>
              <label
                htmlFor='register-last-name'
                className='block text-sm font-medium text-gray-700 mb-1'>
                Last Name *
              </label>
              <input
                id='register-last-name'
                type='text'
                name='last_name'
                value={formData.last_name}
                onChange={handleChange}
                className={`w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 ${
                  errors.last_name ? "border-red-500" : "border-gray-300"
                }`}
                placeholder='Last name'
              />
              {errors.last_name && (
                <p className='text-red-500 text-sm mt-1'>{errors.last_name}</p>
              )}
            </div>
          </div>

          <div>
            <label
              htmlFor='register-dob'
              className='block text-sm font-medium text-gray-700 mb-1'>
              Date of Birth *
            </label>
            <input
              id='register-dob'
              type='date'
              name='date_of_birth'
              value={formData.date_of_birth}
              onChange={handleChange}
              max={todayIsoDate}
              className={`w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 ${
                errors.date_of_birth ? "border-red-500" : "border-gray-300"
              }`}
            />
            {errors.date_of_birth && (
              <p className='text-red-500 text-sm mt-1'>
                {errors.date_of_birth}
              </p>
            )}
          </div>

          <div>
            <label
              htmlFor='register-password'
              className='block text-sm font-medium text-gray-700 mb-1'>
              Password *
            </label>
            <input
              id='register-password'
              type='password'
              name='password'
              autoComplete='new-password'
              value={formData.password}
              onChange={handleChange}
              className={`w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 ${
                errors.password ? "border-red-500" : "border-gray-300"
              }`}
              placeholder='12 to 15 characters'
            />
            {errors.password && (
              <p className='text-red-500 text-sm mt-1'>{errors.password}</p>
            )}
            <PasswordRequirements password={formData.password} />
          </div>

          <div>
            <label
              htmlFor='register-confirm-password'
              className='block text-sm font-medium text-gray-700 mb-1'>
              Confirm Password *
            </label>
            <input
              id='register-confirm-password'
              type='password'
              name='confirmPassword'
              autoComplete='new-password'
              value={formData.confirmPassword}
              onChange={handleChange}
              className={`w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 ${
                errors.confirmPassword ? "border-red-500" : "border-gray-300"
              }`}
              placeholder='Confirm your password'
            />
            {errors.confirmPassword && (
              <p className='text-red-500 text-sm mt-1'>
                {errors.confirmPassword}
              </p>
            )}
          </div>

          <div>
            <label
              htmlFor='register-nickname'
              className='block text-sm font-medium text-gray-700 mb-1'>
              Nickname <span className='text-gray-400 text-xs'>(Optional)</span>
            </label>
            <input
              id='register-nickname'
              type='text'
              name='nickname'
              value={formData.nickname}
              onChange={handleNicknameChange}
              maxLength={14}
              className='w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500'
              placeholder='3-14 characters, letters and numbers'
            />
            {nicknameStatus === "checking" && (
              <p className='text-gray-500 text-xs mt-1'>
                ○ Checking availability...
              </p>
            )}
            {nicknameStatus === "available" && (
              <p className='text-green-700 text-xs mt-1'>
                ✓ Nickname is available
              </p>
            )}
            {nicknameStatus === "taken" && (
              <p className='text-red-500 text-xs mt-1'>
                ✕ Nickname is already taken
              </p>
            )}
            {nicknameStatus === "invalid" && (
              <p className='text-red-500 text-xs mt-1'>
                ✕ 3-14 characters, letters and numbers only
              </p>
            )}
            {nicknameStatus === "error" && (
              <p className='text-red-500 text-xs mt-1'>
                ✕ Could not check availability - try again
              </p>
            )}
            {/* "invalid"/"taken"/"error" already have their own message
                above; only "checking" needs this one, to explain why
                submitting while the check is still in flight was blocked. */}
            {errors.nickname && nicknameStatus === "checking" && (
              <p className='text-red-500 text-sm mt-1'>{errors.nickname}</p>
            )}
          </div>

          <div>
            <label
              htmlFor='register-about-me'
              className='block text-sm font-medium text-gray-700 mb-1'>
              About Me <span className='text-gray-400 text-xs'>(Optional)</span>
            </label>
            <textarea
              id='register-about-me'
              name='about_me'
              value={formData.about_me}
              onChange={handleChange}
              rows='3'
              className='w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 resize-none'
              placeholder='Tell us about yourself...'
            />
          </div>

          <div>
            {/* A group heading for the toggle buttons below, not a single
                form field, so this is a <p> rather than a <label>. */}
            <p className='block text-sm font-medium text-gray-700 mb-1'>
              Avatar/Image <span className='text-gray-400 text-xs'>(Optional)</span>
            </p>

            <div className='flex gap-2 mb-2'>
              {[
                { value: "none", label: "No avatar" },
                { value: "dicebear", label: "Choose an avatar" },
                { value: "upload", label: "Upload a photo" },
              ].map((option) => (
                <button
                  key={option.value}
                  type='button'
                  onClick={() => setAvatarChoice(option.value)}
                  className={`px-3 py-1 text-sm rounded-lg border transition ${
                    avatarChoice === option.value
                      ? "bg-blue-600 text-white border-blue-600"
                      : "border-gray-300 text-gray-700 hover:bg-gray-50"
                  }`}>
                  {option.label}
                </button>
              ))}
            </div>

            {avatarChoice === "dicebear" && (
              <div className='grid grid-cols-3 gap-2'>
                {DICEBEAR_AVATARS.map((avatar) => (
                  <img
                    key={avatar.seed}
                    src={avatar.url}
                    alt={`Avatar option: ${avatar.seed}`}
                    onClick={() => setDicebearUrl(avatar.url)}
                    className={`w-12 h-12 rounded-full cursor-pointer border-4 ${
                      dicebearUrl === avatar.url
                        ? "border-blue-500"
                        : "border-white"
                    }`}
                  />
                ))}
              </div>
            )}

            {avatarChoice === "upload" && (
              <div className='flex items-center gap-3'>
                <label className='cursor-pointer px-3 py-2 border border-gray-300 rounded-lg text-sm text-gray-700 hover:bg-gray-50 transition'>
                  Choose file
                  <input
                    id='register-avatar-file'
                    name='avatar'
                    type='file'
                    accept='.jpg,.jpeg,.png,.gif,image/jpeg,image/png,image/gif'
                    onChange={handleAvatarFileChange}
                    className='hidden'
                  />
                </label>
                {avatarPreviewUrl && (
                  <img
                    src={avatarPreviewUrl}
                    alt='Avatar preview'
                    className='w-12 h-12 rounded-full object-cover border'
                  />
                )}
              </div>
            )}
            {avatarChoice === "upload" && (
              <p className='text-gray-400 text-xs mt-1'>
                JPG, PNG, or GIF only - resized automatically before upload.
              </p>
            )}
            {avatarError && (
              <p className='text-red-500 text-sm mt-1'>{avatarError}</p>
            )}
          </div>

          {debugInfo && (
            <div className='text-blue-500 text-xs text-center break-all'>
              {debugInfo}
            </div>
          )}

          {serverError && (
            <div className='text-red-500 text-sm text-center'>
              {serverError}
            </div>
          )}

          <button
            type='submit'
            disabled={isLoading}
            className='w-full bg-blue-600 text-white py-2 px-4 rounded-lg hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 transition disabled:opacity-50 disabled:cursor-not-allowed'>
            {isLoading ? "Creating account..." : "Create Account"}
          </button>
        </form>

        <div className='mt-6 text-center'>
          <p className='text-sm text-gray-600'>
            Already have an account?{" "}
            <Link
              href='/login'
              className='text-blue-600 hover:text-blue-800 font-medium'>
              Sign in
            </Link>
          </p>
        </div>
      </div>
    </div>
  );
}
