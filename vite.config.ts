import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { defineConfig } from "vite";
import { barefoot } from "@barefootjs/go-template/vite";

const HERE = dirname(fileURLToPath(import.meta.url));

// Compiled client bundles are served from /client/ (see main.go's
// r.Static("/client", "dist/client") / mux.Handle("/client/", ...)).
export default defineConfig({
  base: "/client/",
  resolve: {
    alias: {
      // Mirrors tsconfig.json's `@/components/*` path mapping — `tsc`/
      // `tsx` read that natively, but Vite's dev-server dependency
      // pre-scan (esbuild, run before this plugin's own `transform`
      // hook ever sees the file) parses raw source directly and has no
      // notion of tsconfig `paths` without this. The registry's
      // <Button>'s `import { Slot } from '../slot'`-style sibling
      // imports resolve fine without it (plain relative paths); only
      // the `@/components/...`-style ones used by the starter Counter
      // need this.
      "@/components": resolve(HERE, "components"),
    },
  },
  // `./public` is served directly by main.go (r.Static("/static",
  // "public")), not by Vite — Vite's own default `publicDir` behavior
  // (copy it verbatim into `build.outDir`) would otherwise write a
  // second, build-order-dependent copy under `dist/client` that main.go
  // never reads (it only ever serves `dist/client` at `/client/`), and
  // that copy runs stale the moment `unocss` (which regenerates
  // `public/uno.css`) runs AFTER this build in `package.json`'s `build`
  // script.
  publicDir: false,
  build: {
    outDir: "dist/client",
    emptyOutDir: true,
  },
  plugins: barefoot({
    components: ["components"],
    // Compiled Go html/template files land here — main.go's
    // loadTemplates() walks this directory recursively.
    templates: "dist/templates",
    packageName: "main",
    // Generated Go struct types for every component, written next to
    // main.go. Overwritten on every `vite build` / `vite dev` pass.
    typesOutputFile: "components.go",
  }),
});
