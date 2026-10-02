import { cloneElement, isValidElement, useEffect, useLayoutEffect, useRef, useState, type CSSProperties, type ReactElement, type ReactNode } from "react";
import { createPortal } from "react-dom";

export type TooltipProps = {
  /** Base for the bubble id the trigger's aria-describedby points at. Unique per page. */
  id: string;
  text: string;
  placement?: "top" | "bottom";
  /** One focusable element. */
  children: ReactNode;
};

const edge = 8;

// The design Tooltip positions its bubble inside the trigger, so overflow containers clip it
// or grow scrollbars. This one portals the bubble to body with fixed coordinates and keeps the
// design classes. Stopgap until https://github.com/Xaroth/design/issues/12.
export function Tooltip({ id, text, placement = "top", children }: TooltipProps) {
  const triggerRef = useRef<HTMLSpanElement>(null);
  const bubbleRef = useRef<HTMLSpanElement>(null);
  const [shown, setShown] = useState(false);
  const [style, setStyle] = useState<CSSProperties>({});
  const [side, setSide] = useState(placement);
  const bubbleId = `${id}-tip`;

  // Two passes before paint. First the portal box mirrors the trigger rect, so the design CSS
  // (and the theme's alignment) places the bubble as it would inline. Then the measured bubble
  // is nudged into the viewport and its arrow re-aimed at the trigger.
  useLayoutEffect(() => {
    const t = triggerRef.current?.getBoundingClientRect();
    if (!shown || !t) return;
    setStyle({ position: "fixed", top: t.top, left: t.left, width: t.width, height: t.height, pointerEvents: "none", zIndex: 1000 });
    setSide(placement);
  }, [shown, placement, text]);

  useLayoutEffect(() => {
    const t = triggerRef.current?.getBoundingClientRect();
    const b = bubbleRef.current?.getBoundingClientRect();
    if (!shown || !t || !b || style.translate !== undefined || style.position === undefined) return;
    const fits = side === "top" ? b.top >= edge : b.bottom <= window.innerHeight - edge;
    // Flip once; if neither side fits, keep the requested one.
    if (!fits && side === placement) {
      setSide(side === "top" ? "bottom" : "top");
      return;
    }
    const dx = Math.min(Math.max(b.left, edge), window.innerWidth - edge - b.width) - b.left;
    const arrow = t.left + t.width / 2 - (b.left + dx);
    setStyle({ ...style, translate: `${dx}px 0`, ["--x-tooltip-arrow-left" as string]: `${arrow}px` });
  }, [shown, style, side, placement]);

  useEffect(() => {
    if (!shown) return;
    const hide = () => setShown(false);
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && hide();
    // Hide rather than follow: the trigger may scroll out of view.
    window.addEventListener("scroll", hide, true);
    window.addEventListener("resize", hide);
    document.addEventListener("keydown", onKey);
    return () => {
      window.removeEventListener("scroll", hide, true);
      window.removeEventListener("resize", hide);
      document.removeEventListener("keydown", onKey);
    };
  }, [shown]);

  const trigger = isValidElement(children)
    ? cloneElement(children as ReactElement<{ "aria-describedby"?: string }>, {
        "aria-describedby": [(children as ReactElement<{ "aria-describedby"?: string }>).props["aria-describedby"], bubbleId].filter(Boolean).join(" "),
      })
    : children;

  return (
    <span
      ref={triggerRef}
      style={{ display: "inline-block" }}
      onPointerEnter={() => setShown(true)}
      onPointerLeave={() => setShown(false)}
      onFocus={(e) => e.target.matches(":focus-visible") && setShown(true)}
      onBlur={() => setShown(false)}
    >
      {trigger}
      {/* Always mounted so aria-describedby resolves while hidden. */}
      {createPortal(
        <span className={`x-tooltip x-tooltip--${side}${shown ? " x-tooltip--open" : ""}`} style={style}>
          <span ref={bubbleRef} className="x-tooltip__bubble" role="tooltip" id={bubbleId}>
            {text}
          </span>
        </span>,
        document.body,
      )}
    </span>
  );
}
