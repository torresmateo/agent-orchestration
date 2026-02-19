// Theme colors for the TUI
export const colors = {
  bg: "black",
  fg: "white",
  border: "cyan",
  borderFocus: "green",
  title: "bold",

  // Agent states
  stateColors: {
    starting: "yellow",
    cloning: "yellow",
    executing: "green",
    pushing: "blue",
    serving: "magenta",
    completed: "cyan",
    failed: "red",
    killed: "red",
    registered: "white",
    running: "green",
    active: "green",
  } as Record<string, string>,

  // Pool metrics
  poolWarm: "green",
  poolActive: "yellow",
  poolCold: "blue",

  // Status
  connected: "green",
  disconnected: "red",

  // Tools
  toolColors: {
    "claude-code": "magenta",
    opencode: "cyan",
    amp: "yellow",
    cline: "green",
  } as Record<string, string>,
} as const;

export function stateColor(state: string): string {
  return colors.stateColors[state] || "white";
}

export function toolColor(tool: string): string {
  return colors.toolColors[tool] || "white";
}

export function stateIcon(state: string): string {
  const icons: Record<string, string> = {
    starting: "...",
    cloning: ">>>",
    executing: "***",
    pushing: "^^^",
    serving: "~~~",
    completed: "[+]",
    failed: "[X]",
    killed: "[X]",
    registered: "[ ]",
    running: "[>]",
    active: "[>]",
  };
  return icons[state] || "[ ]";
}
