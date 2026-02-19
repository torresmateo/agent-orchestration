import { createApp } from "./app.js";

const WS_URL = process.env.AGENTVM_WS_URL || "ws://127.0.0.1:8091/ws";

console.log("agentvm-monitor starting...");
console.log(`Connecting to ${WS_URL}`);

const { screen } = createApp({ wsUrl: WS_URL });

// Handle uncaught errors gracefully
process.on("uncaughtException", (err) => {
  screen.destroy();
  console.error("Fatal error:", err);
  process.exit(1);
});

process.on("unhandledRejection", (err) => {
  // Log but don't crash for unhandled promise rejections
  // (e.g. failed fetch calls when agentd is down)
});
