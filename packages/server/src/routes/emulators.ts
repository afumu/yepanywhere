import { Hono } from "hono";
import type { EmulatorBridgeService } from "../emulator/EmulatorBridgeService.js";

interface EmulatorRoutesDeps {
  emulatorBridgeService: EmulatorBridgeService;
}

/**
 * Creates emulator-related API routes.
 *
 * GET  /api/emulators                  - List all emulators (running + stopped AVDs)
 * POST /api/emulators/:id/start        - Start a stopped emulator
 * POST /api/emulators/:id/stop         - Stop a running emulator
 * GET  /api/emulators/:id/screenshot   - Get a JPEG screenshot thumbnail
 * POST /api/emulators/bridge/download  - Download the bridge binary from GitHub
 */
export function createEmulatorRoutes(deps: EmulatorRoutesDeps): Hono {
  const { emulatorBridgeService } = deps;
  const routes = new Hono();

  // POST /api/emulators/bridge/download - Download bridge binary
  routes.post("/bridge/download", async (c) => {
    try {
      const destPath = await emulatorBridgeService.downloadBinary();
      return c.json({ ok: true, path: destPath });
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      console.error("[EmulatorRoutes] POST /bridge/download error:", message);
      return c.json({ ok: false, error: message }, 500);
    }
  });

  // GET /api/emulators - List emulators
  routes.get("/", async (c) => {
    try {
      const emulators = await emulatorBridgeService.listEmulators();
      return c.json(emulators);
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      console.error("[EmulatorRoutes] GET /emulators error:", message);
      return c.json({ error: message }, 500);
    }
  });

  // POST /api/emulators/:id/start
  routes.post("/:id/start", async (c) => {
    const id = c.req.param("id");
    try {
      await emulatorBridgeService.startEmulator(id);
      return c.json({ ok: true });
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      console.error(
        `[EmulatorRoutes] POST /emulators/${id}/start error:`,
        message,
      );
      return c.json({ error: message }, 500);
    }
  });

  // POST /api/emulators/:id/stop
  routes.post("/:id/stop", async (c) => {
    const id = c.req.param("id");
    try {
      await emulatorBridgeService.stopEmulator(id);
      return c.json({ ok: true });
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      console.error(
        `[EmulatorRoutes] POST /emulators/${id}/stop error:`,
        message,
      );
      return c.json({ error: message }, 500);
    }
  });

  // GET /api/emulators/:id/screenshot
  routes.get("/:id/screenshot", async (c) => {
    const id = c.req.param("id");
    try {
      const jpeg = await emulatorBridgeService.getScreenshot(id);
      return new Response(new Uint8Array(jpeg), {
        headers: { "Content-Type": "image/jpeg" },
      });
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      console.error(
        `[EmulatorRoutes] GET /emulators/${id}/screenshot error:`,
        message,
      );
      return c.json({ error: message }, 500);
    }
  });

  return routes;
}
