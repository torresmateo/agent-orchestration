import { exec } from "child_process";
import type { Store } from "../state/store.js";
import type { WSClient } from "../connection/ws-client.js";

// Mount/unmount an agent's workspace via SSHFS.
export async function toggleMount(
  store: Store,
  wsClient: WSClient
): Promise<{ mounted: boolean; path?: string; error?: string }> {
  const agent = store.selectedAgent;
  if (!agent) return { mounted: false, error: "No agent selected" };

  // Try mount first (server will return already-mounted path if already mounted)
  const result = await wsClient.command("mount", {
    agentID: agent.agentID,
  });

  if (result.success) {
    return {
      mounted: true,
      path: result.message,
    };
  }

  // If mount failed, try unmount (it might already be mounted and we're toggling)
  if (result.error?.includes("already")) {
    const unmountResult = await wsClient.command("unmount", {
      agentID: agent.agentID,
    });
    return {
      mounted: false,
      error: unmountResult.success ? undefined : unmountResult.error,
    };
  }

  return { mounted: false, error: result.error };
}

// Open a mounted workspace in VS Code.
export function openInVSCode(mountPath: string): Promise<string | null> {
  return new Promise((resolve) => {
    exec(`code "${mountPath}"`, (err) => {
      if (err) {
        resolve(`Failed to open VS Code: ${err.message}`);
      } else {
        resolve(null);
      }
    });
  });
}
