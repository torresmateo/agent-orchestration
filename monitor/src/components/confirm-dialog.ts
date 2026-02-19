import blessed from "blessed";
import { colors } from "../utils/colors.js";

export function showConfirmDialog(
  screen: blessed.Widgets.Screen,
  message: string,
  onConfirm: () => void
): void {
  const box = blessed.box({
    parent: screen,
    top: "center",
    left: "center",
    width: 50,
    height: 7,
    label: " Confirm ",
    border: { type: "line" },
    tags: true,
    style: {
      fg: colors.fg,
      bg: "black",
      border: { fg: "yellow" },
      label: { fg: "yellow" },
    },
  });

  blessed.text({
    parent: box,
    top: 1,
    left: 2,
    content: message,
    tags: true,
    style: { fg: "white", bg: "black" },
  });

  const yesBtn = blessed.button({
    parent: box,
    top: 3,
    left: 8,
    width: 10,
    height: 1,
    content: " Yes (y) ",
    style: {
      fg: "white",
      bg: "red",
      focus: { bg: "blue" },
    },
    mouse: true,
  });

  const noBtn = blessed.button({
    parent: box,
    top: 3,
    left: 22,
    width: 10,
    height: 1,
    content: " No (n) ",
    style: {
      fg: "white",
      bg: "green",
      focus: { bg: "blue" },
    },
    mouse: true,
  });

  function close() {
    box.destroy();
    screen.render();
  }

  noBtn.on("press", close);
  yesBtn.on("press", () => {
    close();
    onConfirm();
  });

  box.key(["y", "enter"], () => {
    close();
    onConfirm();
  });

  box.key(["n", "escape"], close);

  box.focus();
  screen.render();
}
