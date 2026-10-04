// The sprites a ship or group swaps between as its status changes (locked, unlocked, mastery level). Loading and
// decoding them up front stops a character switch from popping them in one by one. Names follow the package's
// src/ship-tree sprite imports.
const sprites = import.meta.glob<string>(
  "/node_modules/@eve-online-tools/eve-ship-tree/dist/esm/_virtual/{bgfill,bgvignette,bottomleft,bottomline,bottomseperator,elite,frameelite,framelocked,framelower,frameunlocked,frameupper,groupiconframe,locked,masterysmall0,masterysmall1,masterysmall2,masterysmall3,masterysmall4,masterysmall5,navy,omega_64,tech2,tech3,topleft,topright,unlocked}.png.mjs",
  { eager: true, import: "default" },
);

// Held so the decoded images stay in memory.
const images: HTMLImageElement[] = [];

export function preloadSprites() {
  if (images.length > 0) return;
  for (const url of Object.values(sprites)) {
    const img = new Image();
    img.src = url;
    img.decode().catch(() => {});
    images.push(img);
  }
}
