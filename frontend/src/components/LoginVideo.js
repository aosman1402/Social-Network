"use client";

import { useEffect, useState } from "react";

// Matches the breakpoint in globals.css (.login-landing) below which the
// page shows only the login form.
const WIDE_ENOUGH = "(min-width: 768px)";

// The looping intro video beside the login form. Only rendered on screens
// wide enough to show it, so phones never download the ~8 MB file (a video
// merely hidden with CSS would still be fetched). Visitors who ask for
// reduced motion get the still poster frame instead of the animation.
export default function LoginVideo() {
  const [show, setShow] = useState(false);
  const [reduceMotion, setReduceMotion] = useState(false);

  useEffect(() => {
    const wide = window.matchMedia(WIDE_ENOUGH);
    const motion = window.matchMedia("(prefers-reduced-motion: reduce)");
    const update = () => {
      setShow(wide.matches);
      setReduceMotion(motion.matches);
    };
    update();
    wide.addEventListener("change", update);
    motion.addEventListener("change", update);
    return () => {
      wide.removeEventListener("change", update);
      motion.removeEventListener("change", update);
    };
  }, []);

  if (!show) return null;

  return (
    <div className='login-video' aria-hidden='true'>
      {reduceMotion ? (
        <img src='/videos/login-loop-poster.jpg' alt='' />
      ) : (
        <video
          src='/videos/login-loop.mp4'
          poster='/videos/login-loop-poster.jpg'
          autoPlay
          muted
          loop
          playsInline
          preload='auto'
        />
      )}
    </div>
  );
}
