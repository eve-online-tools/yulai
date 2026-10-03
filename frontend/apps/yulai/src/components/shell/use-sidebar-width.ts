import { useEffect, useRef, useState, type KeyboardEvent, type PointerEvent } from "react";

const COLLAPSED = 60;
// Narrow enough to still show part of a label, so the user can tell it opens further.
const MIN = 140;
const MAX = 360;
const DEFAULT = 220;
const STEP = 16;
const storageKey = "yulai.sidebarWidth";

// Below the midpoint the sidebar snaps to icons only; above it, it never gets narrower than MIN.
function snap(w: number) {
  if (w < (COLLAPSED + MIN) / 2) return COLLAPSED;
  return Math.min(MAX, Math.max(MIN, Math.round(w)));
}

function load() {
  const w = Number(localStorage.getItem(storageKey));
  return w ? snap(w) : DEFAULT;
}

export function useSidebarWidth() {
  const [width, setWidth] = useState(load);
  const drag = useRef<{ x: number; width: number } | null>(null);

  useEffect(() => localStorage.setItem(storageKey, String(width)), [width]);

  const handleProps = {
    role: "separator",
    "aria-orientation": "vertical" as const,
    "aria-label": "Resize sidebar",
    "aria-valuemin": COLLAPSED,
    "aria-valuemax": MAX,
    "aria-valuenow": width,
    tabIndex: 0,
    onPointerDown: (e: PointerEvent<HTMLElement>) => {
      if (e.button !== 0) return;
      e.preventDefault();
      e.currentTarget.setPointerCapture(e.pointerId);
      drag.current = { x: e.clientX, width };
    },
    onPointerMove: (e: PointerEvent<HTMLElement>) => {
      if (drag.current) setWidth(snap(drag.current.width + e.clientX - drag.current.x));
    },
    onPointerUp: () => {
      drag.current = null;
    },
    onKeyDown: (e: KeyboardEvent<HTMLElement>) => {
      if (e.key === "ArrowLeft") setWidth((w) => (w <= MIN ? COLLAPSED : snap(w - STEP)));
      else if (e.key === "ArrowRight") setWidth((w) => (w === COLLAPSED ? MIN : snap(w + STEP)));
      else return;
      e.preventDefault();
    },
  };

  return { width, collapsed: width === COLLAPSED, handleProps };
}
