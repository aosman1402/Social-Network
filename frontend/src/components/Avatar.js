import { getInitials, getAvatarColor, getAvatarSrc } from "@/utils/helpers";

export default function Avatar({ user, size = "md" }) {
  const sizeClasses = {
    xs: "w-6 h-6 text-xs",
    sm: "w-8 h-8 text-sm",
    md: "w-10 h-10 text-base",
    lg: "w-12 h-12 text-lg",
    xl: "w-16 h-16 text-xl",
    "2xl": "w-24 h-24 text-3xl",
  };

  // Stable per person (not random on every render)
  const color = getAvatarColor(user?.id ?? `${user?.first_name}${user?.last_name}`);
  const initials = user ? getInitials(user.first_name, user.last_name) : "?";
  const avatarSrc = getAvatarSrc(user?.avatar_path);

  return (
    <div
      className={`${sizeClasses[size]} rounded-full flex items-center justify-center text-white font-bold`}
      style={{ background: color, boxShadow: "0 0 0 2px #fff, 0 2px 6px rgba(15, 23, 42, 0.15)" }}>
      {avatarSrc ? (
        <img
          src={avatarSrc}
          alt='Avatar'
          className='w-full h-full rounded-full object-cover'
        />
      ) : (
        initials
      )}
    </div>
  );
}
