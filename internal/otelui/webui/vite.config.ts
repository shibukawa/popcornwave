import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import { resolve } from "node:path";

// The build output is committed under ../web and embedded by package otelui, so
// a pw release build needs no Node toolchain. Rebuild with `npm run build`
// after taking new component sources from the upstream repository.
//
// `npm run dev` serves the UI alone; point it at a running `pw dev` viewer with
// PW_OTEL_ENDPOINT, whose port changes every run.
export default defineConfig({
  plugins: [react()],
  base: "./",
  build: {
    outDir: resolve(__dirname, "../web"),
    emptyOutDir: true,
  },
  server: {
    proxy: { "/api": process.env.PW_OTEL_ENDPOINT ?? "http://127.0.0.1:4318" },
  },
});
