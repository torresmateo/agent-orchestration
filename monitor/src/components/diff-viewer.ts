import blessed from "blessed";
import type { Store } from "../state/store.js";

// DiffViewer fetches and displays git diff from the agentd REST API.
export class DiffViewer {
  private store: Store;
  private diffBox: blessed.Widgets.BoxElement;
  private screen: blessed.Widgets.Screen;
  private lastAgentId: string | null = null;

  constructor(
    store: Store,
    diffBox: blessed.Widgets.BoxElement,
    screen: blessed.Widgets.Screen
  ) {
    this.store = store;
    this.diffBox = diffBox;
    this.screen = screen;
  }

  async load() {
    const agent = this.store.selectedAgent;
    if (!agent) {
      this.diffBox.setContent("  No agent selected");
      this.screen.render();
      return;
    }

    // Avoid reloading if already loaded for this agent
    if (agent.agentID === this.lastAgentId) return;
    this.lastAgentId = agent.agentID;

    this.diffBox.setContent("  Loading diff...");
    this.screen.render();

    try {
      const resp = await fetch(
        `http://127.0.0.1:8091/agents/${agent.agentID}/diff`
      );
      if (!resp.ok) {
        this.diffBox.setContent(`  Error: HTTP ${resp.status}`);
        this.screen.render();
        return;
      }

      const data = (await resp.json()) as { diff: string; stat: string };
      const lines: string[] = [];

      if (data.stat) {
        lines.push(`{bold}${blessed.escape(data.stat)}{/bold}`);
        lines.push("");
      }

      if (data.diff && data.diff.trim() !== "no changes") {
        for (const line of data.diff.split("\n")) {
          if (line.startsWith("+++") || line.startsWith("---")) {
            lines.push(`{bold}${blessed.escape(line)}{/bold}`);
          } else if (line.startsWith("+")) {
            lines.push(`{green-fg}${blessed.escape(line)}{/green-fg}`);
          } else if (line.startsWith("-")) {
            lines.push(`{red-fg}${blessed.escape(line)}{/red-fg}`);
          } else if (line.startsWith("@@")) {
            lines.push(`{cyan-fg}${blessed.escape(line)}{/cyan-fg}`);
          } else if (line.startsWith("diff ")) {
            lines.push(`{yellow-fg}${blessed.escape(line)}{/yellow-fg}`);
          } else {
            lines.push(blessed.escape(line));
          }
        }
      } else {
        lines.push("  No changes detected");
      }

      this.diffBox.setContent(lines.join("\n"));
      this.screen.render();
    } catch (e: any) {
      this.diffBox.setContent(`  Error: ${e.message}`);
      this.screen.render();
    }
  }

  reset() {
    this.lastAgentId = null;
  }
}
