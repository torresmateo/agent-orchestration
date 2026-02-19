import blessed from "blessed";
import type { Store } from "../state/store.js";

// FileBrowser fetches directory listings and file contents via the agentd REST API.
export class FileBrowser {
  private store: Store;
  private fileBox: blessed.Widgets.BoxElement;
  private screen: blessed.Widgets.Screen;
  private currentPath = ".";
  private pathHistory: string[] = [];

  constructor(
    store: Store,
    fileBox: blessed.Widgets.BoxElement,
    screen: blessed.Widgets.Screen
  ) {
    this.store = store;
    this.fileBox = fileBox;
    this.screen = screen;
  }

  async loadPath(path: string) {
    const agent = this.store.selectedAgent;
    if (!agent) {
      this.fileBox.setContent("  No agent selected");
      this.screen.render();
      return;
    }

    this.fileBox.setContent(`  Loading ${path}...`);
    this.screen.render();

    try {
      const resp = await fetch(
        `http://127.0.0.1:8091/agents/${agent.agentID}/files?path=${encodeURIComponent(path)}`
      );
      if (!resp.ok) {
        this.fileBox.setContent(`  Error: HTTP ${resp.status}`);
        this.screen.render();
        return;
      }

      const data = (await resp.json()) as {
        type: string;
        path: string;
        content: string;
      };

      if (data.type === "directory") {
        this.currentPath = path;
        const lines = [
          `{bold}  Directory: ${data.path}{/bold}`,
          "",
          data.content,
        ];
        this.fileBox.setContent(lines.join("\n"));
      } else {
        this.fileBox.setContent(
          `{bold}  File: ${data.path}{/bold}\n\n${blessed.escape(data.content)}`
        );
      }
      this.screen.render();
    } catch (e: any) {
      this.fileBox.setContent(`  Error: ${e.message}`);
      this.screen.render();
    }
  }

  async refresh() {
    await this.loadPath(this.currentPath);
  }

  async goUp() {
    if (this.currentPath === ".") return;
    this.pathHistory.push(this.currentPath);
    const parts = this.currentPath.split("/");
    parts.pop();
    const parent = parts.join("/") || ".";
    await this.loadPath(parent);
  }

  async goBack() {
    const prev = this.pathHistory.pop();
    if (prev) {
      await this.loadPath(prev);
    }
  }
}
