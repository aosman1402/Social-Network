"use client";

import { resizeImageFile } from "@/utils/helpers";

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
// Exported so other places that let a user pick one of these presets (e.g.
// the Settings "Change Avatar" option) reuse the exact same six instead of
// redefining the seed/color list - the backend validates against this same
// set too (see presetAvatarSeeds/presetAvatarColors in user.go), so drift
// here would make a legitimately-picked avatar fail server-side.
export const DICEBEAR_AVATARS = AVATAR_SEEDS.map((seed, i) => ({
  seed,
  url: `https://api.dicebear.com/10.x/clay/svg?seed=${seed}&backgroundColor=${AVATAR_BACKGROUND_COLORS[i]}`,
}));

// Same allowlist the backend enforces (by sniffing the real file bytes) -
// checked here first so the user gets immediate feedback instead of
// waiting on a round trip for an upload that was always going to be rejected.
const ALLOWED_AVATAR_TYPES = ["image/jpeg", "image/png", "image/gif"];
const ALLOWED_AVATAR_EXTENSIONS = [".jpg", ".jpeg", ".png", ".gif"];

// Lets the user pick "no avatar", one of a handful of DiceBear presets, or
// upload their own photo (validated, then resized client-side before the
// caller ever sees it). The caller only needs the final chosen state -
// preview rendering and file validation/resizing live entirely in here.
export default function AvatarPicker({
  choice,
  onChoiceChange,
  dicebearUrl,
  onDicebearUrlChange,
  avatarPreviewUrl,
  onFileChange, // (resizedFile: Blob | null, previewUrl: string) => void
}) {
  const handleFileChange = async (e) => {
    const file = e.target.files[0];
    if (!file) return;

    const extension = file.name.slice(file.name.lastIndexOf(".")).toLowerCase();
    const isAllowedType =
      ALLOWED_AVATAR_TYPES.includes(file.type) ||
      ALLOWED_AVATAR_EXTENSIONS.includes(extension);

    if (!isAllowedType) {
      onFileChange(null, "", "Please choose a JPG, PNG, or GIF image.");
      e.target.value = "";
      return;
    }
    if (file.size > 5 * 1024 * 1024) {
      onFileChange(null, "", "Image must be less than 5MB.");
      e.target.value = "";
      return;
    }

    try {
      // Shrink dimensions/quality client-side before it ever reaches the
      // server, so a full-resolution photo doesn't end up sitting in storage.
      const resized = await resizeImageFile(file);
      if (avatarPreviewUrl) URL.revokeObjectURL(avatarPreviewUrl);
      onFileChange(resized, URL.createObjectURL(resized), "");
    } catch (err) {
      onFileChange(null, "", "Failed to process that image. Please try a different file.");
    }
  };

  return (
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
            onClick={() => onChoiceChange(option.value)}
            className={`px-3 py-1 text-sm rounded-lg border transition ${
              choice === option.value
                ? "bg-blue-600 text-white border-blue-600"
                : "border-gray-300 text-gray-700 hover:bg-gray-50"
            }`}>
            {option.label}
          </button>
        ))}
      </div>

      {choice === "dicebear" && (
        <div className='grid grid-cols-3 gap-2'>
          {DICEBEAR_AVATARS.map((avatar) => (
            <img
              key={avatar.seed}
              src={avatar.url}
              alt={`Avatar option: ${avatar.seed}`}
              onClick={() => onDicebearUrlChange(avatar.url)}
              className={`w-12 h-12 rounded-full cursor-pointer border-4 ${
                dicebearUrl === avatar.url ? "border-blue-500" : "border-white"
              }`}
            />
          ))}
        </div>
      )}

      {choice === "upload" && (
        <div className='flex items-center gap-3'>
          <label className='cursor-pointer px-3 py-2 border border-gray-300 rounded-lg text-sm text-gray-700 hover:bg-gray-50 transition'>
            Choose file
            <input
              id='register-avatar-file'
              name='avatar'
              type='file'
              accept='.jpg,.jpeg,.png,.gif,image/jpeg,image/png,image/gif'
              onChange={handleFileChange}
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
      {choice === "upload" && (
        <p className='text-gray-400 text-xs mt-1'>
          JPG, PNG, or GIF only - resized automatically before upload.
        </p>
      )}
    </div>
  );
}
