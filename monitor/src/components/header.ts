import blessed from "blessed";
import type { Store } from "../state/store.js";
import { colors } from "../utils/colors.js";

export function createHeader(
  parent: blessed.Widgets.Screen,
  store: Store
): blessed.Widgets.BoxElement {
  const box = blessed.box({
    parent,
    top: 0,
    left: 0,
    right: 0,
    height: 3,
    tags: true,
    style: {
      fg: colors.fg,
      bg: colors.bg,
    },
  });

  function render() {
    const { pool, connected, agents } = store.current;
    const connStatus = connected
      ? `{${colors.connected}-fg}CONNECTED{/}`
      : `{${colors.disconnected}-fg}DISCONNECTED{/}`;

    const poolLine = [
      `{${colors.poolWarm}-fg}${pool.warm} warm{/}`,
      `{${colors.poolActive}-fg}${pool.active} active{/}`,
      `{${colors.poolCold}-fg}${pool.cold} cold{/}`,
    ].join(" | ");

    box.setContent(
      ` {bold}agentvm-monitor{/bold}  ${connStatus}  |  Pool: ${poolLine}  |  Agents: ${agents.length}`
    );
    // render is driven by app-level screen.render()
  }

  store.subscribe(render);
  render();

  return box;
}
