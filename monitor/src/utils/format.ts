// Time formatting helpers
export function formatElapsed(elapsed: string): string {
  // Elapsed comes as Go duration string like "1h23m45s" or "5m30s"
  if (!elapsed || elapsed === "0s") return "just now";
  return elapsed;
}

export function formatTime(iso: string): string {
  if (!iso) return "-";
  try {
    const d = new Date(iso);
    return d.toLocaleTimeString("en-US", {
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
      hour12: false,
    });
  } catch {
    return iso;
  }
}

export function truncate(s: string, n: number): string {
  if (s.length <= n) return s;
  return s.slice(0, n - 3) + "...";
}

export function padRight(s: string, n: number): string {
  if (s.length >= n) return s.slice(0, n);
  return s + " ".repeat(n - s.length);
}

export function padLeft(s: string, n: number): string {
  if (s.length >= n) return s.slice(0, n);
  return " ".repeat(n - s.length) + s;
}
