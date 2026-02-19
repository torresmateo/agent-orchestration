import type { Store } from "../state/store.js";
import type { WSClient } from "../connection/ws-client.js";

// Kill an agent via WebSocket command.
export async function killAgent(
  store: Store,
  wsClient: WSClient
): Promise<string | null> {
  const agent = store.selectedAgent;
  if (!agent) return "No agent selected";

  const result = await wsClient.command("kill", {
    agentID: agent.agentID,
  });

  if (!result.success) {
    return result.error || "Kill failed";
  }

  return null; // success
}
