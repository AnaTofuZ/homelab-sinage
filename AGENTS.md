# Signage UI guidelines

## Target display

- Design for a Lenovo Tab M8 in landscape first. Use an `800 x 500` CSS-pixel viewport as the baseline.
- Also check landscape widths of `900`, `962`, and `1280` px. Portrait must remain usable, but is secondary.
- The landscape dashboard is a kiosk screen: it must not scroll in either direction.

## Layout

- Keep every label, value, and animation inside its panel border at the baseline viewport.
- Preserve the three-column hero and three-panel content grid unless the requested feature requires a structural change.
- Prefer responsive CSS spacing and type sizes at the existing `900px` breakpoint over hiding information.
- Give shrinking grid and flex children `min-width: 0` when their content could force a column wider.
- Keep the news ticker clear of the dashboard content and the device edges.

## Visual style

- Reuse the existing dark industrial palette, acid-green accent, thin borders, monospace data, and Japanese labels.
- Keep primary values readable at arm's length. Compact metadata before shrinking clocks, temperatures, or section headings.
- Preserve `prefers-reduced-motion` behavior and semantic/ARIA labels.

## Verification

- Run `npm run format:check`, `npm run lint`, `npm run typecheck`, and `go test ./...`.
- After changing a BarefootJS component, run `npm run build:ui`, then `gofmt -w components.go`, and commit the regenerated `components.go`; CI verifies that exact sequence.
- Visually inspect the dashboard at `800 x 500`; confirm that the clock, current weather, hourly forecast, cards, and ticker remain within their borders.
- Do not hand-edit `dist/` or generated Go component output. `dist/` is a build artifact; `components.go` is generated but tracked for CI.
