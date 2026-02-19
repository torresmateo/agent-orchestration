import blessed from "blessed";
import type { WSClient } from "../connection/ws-client.js";
import { colors } from "../utils/colors.js";

export interface DispatchFormData {
  project: string;
  repoURL: string;
  tool: string;
  prompt: string;
  issue?: string;
  branch?: string;
  maxTime?: number;
}

export function showDispatchDialog(
  screen: blessed.Widgets.Screen,
  wsClient: WSClient
): void {
  const form = blessed.form({
    parent: screen,
    top: "center",
    left: "center",
    width: 60,
    height: 22,
    label: " Dispatch Agent ",
    border: { type: "line" },
    style: {
      fg: colors.fg,
      bg: "black",
      border: { fg: "green" },
      label: { fg: "green" },
    },
    keys: true,
    vi: true,
    tags: true,
  });

  let yPos = 1;
  const fields: Record<string, blessed.Widgets.TextboxElement> = {};

  function addField(
    label: string,
    name: string,
    defaultVal = ""
  ): blessed.Widgets.TextboxElement {
    blessed.text({
      parent: form,
      top: yPos,
      left: 2,
      content: label,
      style: { fg: "white", bg: "black" },
    });

    const input = blessed.textbox({
      parent: form,
      top: yPos,
      left: 14,
      width: 40,
      height: 1,
      inputOnFocus: true,
      value: defaultVal,
      style: {
        fg: "white",
        bg: "#333333",
        focus: { bg: "#555555" },
      },
    });

    fields[name] = input;
    yPos += 2;
    return input;
  }

  addField("Project:", "project");
  addField("Repo URL:", "repoURL");
  addField("Tool:", "tool", "claude-code");
  addField("Prompt:", "prompt");
  addField("Issue:", "issue");
  addField("Branch:", "branch");
  addField("Max Time:", "maxTime", "30");

  // Buttons
  const submitBtn = blessed.button({
    parent: form,
    top: yPos + 1,
    left: 14,
    width: 12,
    height: 1,
    content: " Submit ",
    style: {
      fg: "white",
      bg: "green",
      focus: { bg: "blue" },
    },
    mouse: true,
  });

  const cancelBtn = blessed.button({
    parent: form,
    top: yPos + 1,
    left: 28,
    width: 12,
    height: 1,
    content: " Cancel ",
    style: {
      fg: "white",
      bg: "red",
      focus: { bg: "blue" },
    },
    mouse: true,
  });

  function close() {
    form.destroy();
    screen.render();
  }

  cancelBtn.on("press", close);

  form.key(["escape"], close);

  submitBtn.on("press", async () => {
    const data: Record<string, unknown> = {
      project: fields.project.getValue(),
      repoURL: fields.repoURL.getValue(),
      tool: fields.tool.getValue() || "claude-code",
      prompt: fields.prompt.getValue(),
    };

    const issue = fields.issue.getValue();
    if (issue) data.issue = issue;

    const branch = fields.branch.getValue();
    if (branch) data.branch = branch;

    const maxTime = parseInt(fields.maxTime.getValue() || "30", 10);
    if (maxTime > 0) data.maxTime = maxTime;

    if (!data.project || !data.repoURL || !data.prompt) {
      // Show error inline
      blessed.message({
        parent: screen,
        top: "center",
        left: "center",
        width: 40,
        height: 5,
        border: { type: "line" },
        style: { fg: "red", bg: "black", border: { fg: "red" } },
      }).display("Project, Repo URL, and Prompt are required.", 3, () => {});
      return;
    }

    close();

    const result = await wsClient.command("dispatch", data);
    if (!result.success) {
      blessed.message({
        parent: screen,
        top: "center",
        left: "center",
        width: 50,
        height: 5,
        border: { type: "line" },
        style: { fg: "red", bg: "black", border: { fg: "red" } },
      }).display(`Dispatch failed: ${result.error}`, 5, () => {});
    }
  });

  screen.render();
  // Focus the first field
  fields.project.focus();
}
