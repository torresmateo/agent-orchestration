import { exec } from "child_process";
import type { Store } from "../state/store.js";
import type { WSClient } from "../connection/ws-client.js";

// Launch an external terminal with `limactl shell <vm>`.
export async function openShell(store: Store, wsClient: WSClient): Promise<string | null> {
  const agent = store.selectedAgent;
  if (!agent) return "No agent selected";

  // Ask the server for the VM name (also validates the agent exists)
  const result = await wsClient.command("shell", { agentID: agent.agentID });
  if (!result.success) {
    return result.error || "Failed to get VM info";
  }

  const vmName = result.message;
  if (!vmName) return "No VM name returned";

  // Open a new terminal window with limactl shell
  const platform = process.platform;
  let cmd: string;

  if (platform === "darwin") {
    // macOS: use Terminal.app
    cmd = `osascript -e 'tell application "Terminal" to do script "limactl shell ${vmName}"'`;
  } else {
    // Linux: try common terminal emulators
    cmd = `x-terminal-emulator -e "limactl shell ${vmName}" 2>/dev/null || xterm -e "limactl shell ${vmName}"`;
  }

  return new Promise((resolve) => {
    exec(cmd, (err) => {
      if (err) {
        resolve(`Failed to open terminal: ${err.message}`);
      } else {
        resolve(null);
      }
    });
  });
}
