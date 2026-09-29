"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { useAuth } from "@/context/AuthContext";
import { toast } from "@/context/ToastContext";
import { isValidEmail, isPasswordValid } from "@/utils/helpers";
import PasswordRequirements from "@/components/PasswordRequirements";
import AvatarPicker from "@/components/AvatarPicker";
import GitHubLoginButton from "@/components/GitHubLoginButton";
import NicknameField from "@/components/NicknameField";
import { apiFetch } from "@/lib/api";

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

  // Avatar/Image (Optional): "none" | "dicebear" | "upload"
  const [avatarChoice, setAvatarChoice] = useState("none");
  const [dicebearUrl, setDicebearUrl] = useState("");
  const [avatarFile, setAvatarFile] = useState(null); // resized Blob, ready to upload
  const [avatarPreviewUrl, setAvatarPreviewUrl] = useState("");
  const [avatarError, setAvatarError] = useState("");

  // Nickname (Optional): "idle" | "invalid" | "checking" | "available" | "taken" | "error"
  const [nicknameStatus, setNicknameStatus] = useState("idle");

  const handleNicknameValueChange = (value) => {
    setFormData((prev) => ({ ...prev, nickname: value }));
    if (errors.nickname) setErrors((prev) => ({ ...prev, nickname: "" }));
    setServerError("");
  };

  const handleAvatarFileChange = (file, previewUrl, errorMsg) => {
    setAvatarFile(file);
    setAvatarPreviewUrl(previewUrl);
    setAvatarError(errorMsg);
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
  };

  const handleSubmit = async (e) => {
    e.preventDefault();
    if (!validateForm()) return;

    setIsLoading(true);
    setServerError("");

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

      const { ok, data } = await apiFetch("/register", {
        method: "POST",
        json: payload,
      });

      if (!ok) {
        throw new Error(data.message || data.error || "Registration failed");
      }

      // ✅ Try to auto-login
      const loginResult = await login(formData.email, formData.password);

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
            const { ok: avatarOk, data: avatarData } = await apiFetch(
              "/user/avatar",
              { method: "POST", body: avatarForm }
            );
            if (!avatarOk) {
              toast.error(
                avatarData?.error ||
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

        // Force redirect with window.location
        window.location.href = "/";
      } else {
        console.error("Auto-login failed:", loginResult.error);
        // Fallback: redirect to login page
        router.push("/login?registered=true");
      }
    } catch (error) {
      console.error("Registration error:", error);
      setServerError(error.message || "Registration failed. Please try again.");
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

        <GitHubLoginButton
          label='Sign up with GitHub'
          divider='or with your email'
          dividerPosition='after'
        />

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
              autoComplete='email'
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
                autoComplete='given-name'
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
                autoComplete='family-name'
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
              autoComplete='off'
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

          <NicknameField
            id='register-nickname'
            value={formData.nickname}
            onValueChange={handleNicknameValueChange}
            status={nicknameStatus}
            onStatusChange={setNicknameStatus}
            error={errors.nickname}
          />

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
            <AvatarPicker
              choice={avatarChoice}
              onChoiceChange={setAvatarChoice}
              dicebearUrl={dicebearUrl}
              onDicebearUrlChange={setDicebearUrl}
              avatarPreviewUrl={avatarPreviewUrl}
              onFileChange={handleAvatarFileChange}
            />
            {avatarError && (
              <p className='text-red-500 text-sm mt-1'>{avatarError}</p>
            )}
          </div>

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
