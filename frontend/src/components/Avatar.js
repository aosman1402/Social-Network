import { getInitials, getRandomColor } from "@/utils/helpers";

export default function Avatar({ user, size = "md" }) {
  const sizeClasses = {
    xs: "w-6 h-6 text-xs",
    sm: "w-8 h-8 text-sm",
    md: "w-10 h-10 text-base",
    lg: "w-12 h-12 text-lg",
    xl: "w-16 h-16 text-xl",
  };

  const color = getRandomColor();
  const initials = user ? getInitials(user.first_name, user.last_name) : "?";

  return (
    <div
      className={`${sizeClasses[size]} rounded-full flex items-center justify-center text-white font-bold`}
      style={{ backgroundColor: color }}>
      {user?.avatar_url ? (
        <img
          src={user.avatar_url}
          alt='Avatar'
          className='w-full h-full rounded-full object-cover'
        />
      ) : (
        initials
      )}
    </div>
  );
}
