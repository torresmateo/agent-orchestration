import { spawn } from "child_process";
import type blessed from "blessed";
import type { Store } from "../state/store.js";
import type { WSClient } from "../connection/ws-client.js";

// Yield the terminal to `limactl shell <vm>`, then restore the TUI on exit.
export async function openShell(
  store: Store,
  wsClient: WSClient,
  screen: blessed.Widgets.Screen
): Promise<string | null> {
  const agent = store.selectedAgent;
  if (!agent) return "No agent selected";

  const result = await wsClient.command("shell", { agentID: agent.agentID });
  if (!result.success) {
    return result.error || "Failed to get VM info";
  }

  const vmName = result.message;
  if (!vmName) return "No VM name returned";

  const prog = (screen as any).program;
  const input: any = prog.input;

  return new Promise((resolve) => {
    // 1. Suppress screen.render() — WS updates keep firing and would
    //    write blessed escape sequences over the shell
    const savedRender = screen.render;
    screen.render = () => {};

    // 2. Stop blessed from reading/intercepting stdin
    const wasRaw = input.isRaw;
    input.setRawMode(false);
    input.pause();

    // 3. Exit alternate screen buffer, show cursor
    prog.normalBuffer();
    prog.showCursor();

    // 4. Clear the screen so the shell starts clean
    process.stdout.write("\x1b[2J\x1b[H");

    const child = spawn("limactl", ["shell", vmName], {
      stdio: [0, 1, 2],
    });

    function restore() {
      // Re-enter blessed's alternate screen
      prog.alternateBuffer();
      prog.hideCursor();

      // Resume blessed's input handling
      if (wasRaw) input.setRawMode(true);
      input.resume();

      // Restore screen.render and force full redraw
      screen.render = savedRender;
      (screen as any).alloc();
      screen.render();
    }

    child.on("exit", () => {
      restore();
      resolve(null);
    });

    child.on("error", (err) => {
      restore();
      resolve(`Failed to spawn shell: ${err.message}`);
    });
  });
}
