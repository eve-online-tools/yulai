import { useLayoutEffect, useState, type RefObject } from "react";

export type Anchor = { top: number; left: number; right: number };

// Margin between the anchored controls and the frame, or the pane edge once the frame is out of view.
const gap = 12;

// Places controls just inside the ship tree's frame, below its header. The pan-zoom scales and centres the frame,
// so its corners depend on the window shape; it follows pans and zooms, and keeps to the pane when the frame
// leaves it.
export function useFrameAnchor(pageRef: RefObject<HTMLElement | null>, headerClass: string): Anchor | null {
  const [anchor, setAnchor] = useState<Anchor | null>(null);

  useLayoutEffect(() => {
    const page = pageRef.current;
    if (!page) return;
    let frame = 0;
    const measure = () => {
      cancelAnimationFrame(frame);
      frame = requestAnimationFrame(() => {
        const header = page.querySelector(`.${headerClass}`);
        if (!header) return setAnchor(null);
        const p = page.getBoundingClientRect();
        const h = header.getBoundingClientRect();
        setAnchor({
          top: Math.max(gap, h.bottom - p.top + gap),
          left: Math.max(gap, h.left - p.left + gap),
          right: Math.max(gap, p.right - h.right + gap),
        });
      });
    };
    const resize = new ResizeObserver(measure);
    resize.observe(page);
    // Pan, zoom and the initial fit all write the content transform.
    const transform = new MutationObserver(measure);
    transform.observe(page, { subtree: true, childList: true, attributeFilter: ["style"] });
    measure();
    return () => {
      cancelAnimationFrame(frame);
      resize.disconnect();
      transform.disconnect();
    };
  }, [pageRef, headerClass]);

  return anchor;
}
