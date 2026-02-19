import blessed from "blessed";
import type { Store } from "../state/store.js";
import { colors } from "../utils/colors.js";

export function createStatusBar(
  parent: blessed.Widgets.Screen,
  store: Store
): blessed.Widgets.BoxElement {
  const bar = blessed.box({
    parent,
    bottom: 0,
    left: 0,
    right: 0,
    height: 3,
    tags: true,
    border: { type: "line" },
    style: {
      fg: colors.fg,
      bg: colors.bg,
      border: { fg: colors.border },
    },
  });

  function render() {
    const agent = store.selectedAgent;
    const { activeTab } = store.current;

    const keybindings = [
      "{bold}q{/bold}:Quit",
      "{bold}d{/bold}:Dispatch",
      "{bold}Tab{/bold}:Focus",
      "{bold}j/k{/bold}:Navigate",
      "{bold}1-4{/bold}:Tabs",
      "{bold}K{/bold}:Kill",
      "{bold}m{/bold}:Mount",
      "{bold}s{/bold}:Shell",
      "{bold}v{/bold}:VS Code",
    ];

    if (activeTab === 1) {
      keybindings.push("{bold}f{/bold}:Follow");
    }

    let content = keybindings.join("  ");

    if (agent) {
      content += `\n Selected: {bold}${agent.agentID}{/bold} (${agent.project})`;
    }

    bar.setContent(content);
    // render driven by app-level screen.render()
  }

  store.subscribe(render);
  render();

  return bar;
}
