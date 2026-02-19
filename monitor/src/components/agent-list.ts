import blessed from "blessed";
import type { Store } from "../state/store.js";
import { colors, stateColor, stateIcon, toolColor } from "../utils/colors.js";
import { truncate } from "../utils/format.js";

export function createAgentList(
  parent: blessed.Widgets.Screen,
  store: Store
): blessed.Widgets.ListElement {
  const list = blessed.list({
    parent,
    top: 3,
    left: 0,
    width: Math.floor((parent.width as number) * 0.4),
    bottom: 3,
    label: " Agents ",
    border: { type: "line" },
    scrollbar: {
      ch: " ",
      style: { bg: "cyan" },
    },
    style: {
      fg: colors.fg,
      bg: colors.bg,
      border: { fg: colors.border },
      selected: { bg: "blue", fg: "white", bold: true },
      item: { fg: colors.fg },
      label: { fg: colors.border },
    } as any,
    keys: true,
    vi: true,
    mouse: true,
    tags: true,
  });

  // Track displayed agent IDs for selection mapping
  let displayedAgentIds: string[] = [];

  function render() {
    const byProject = store.agentsByProject;
    const items: string[] = [];
    displayedAgentIds = [];

    if (byProject.size === 0) {
      items.push("  No agents running");
      list.setItems(items);
      return;
    }

    // Sort projects alphabetically
    const projects = [...byProject.keys()].sort();

    for (const project of projects) {
      const agents = byProject.get(project)!;
      items.push(`{bold}{underline}${project}{/underline}{/bold}`);
      displayedAgentIds.push(""); // header row, not selectable

      for (const agent of agents) {
        const icon = stateIcon(agent.state);
        const sColor = stateColor(agent.state);
        const tColor = toolColor(agent.tool);
        const id = truncate(agent.agentID, 15);
        const tool = agent.tool || "?";
        const state = agent.state || "?";
        const elapsed = agent.elapsed || "";

        items.push(
          ` {${sColor}-fg}${icon}{/} ${id} {${tColor}-fg}${tool}{/} {${sColor}-fg}${state}{/} ${elapsed}`
        );
        displayedAgentIds.push(agent.agentID);
      }
    }

    // Preserve selection position
    const selectedIdx = (list as any).selected || 0;
    list.setItems(items);

    // Restore selection
    if (selectedIdx < items.length) {
      list.select(selectedIdx);
    }
  }

  // Handle selection changes
  list.on("select item", (_item: any, index: number) => {
    const agentId = displayedAgentIds[index];
    if (agentId) {
      store.selectAgent(agentId);
    }
  });

  // Also handle move (arrow keys, j/k)
  list.on("keypress", (_ch: string, _key: any) => {
    // After movement, update selection
    setTimeout(() => {
      const idx = (list as any).selected || 0;
      const agentId = displayedAgentIds[idx];
      if (agentId) {
        store.selectAgent(agentId);
      }
    }, 0);
  });

  store.subscribe(render);
  render();

  return list;
}
