import { useEffect, useState } from "react";

// The current time, updated every interval ms.
export function useNow(interval: number) {
  const [now, setNow] = useState(Date.now);
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), interval);
    return () => clearInterval(id);
  }, [interval]);
  return now;
}
